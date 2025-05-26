// Copyright 2025 NVIDIA CORPORATION & AFFILIATES
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

/*----------------------------------------------------

  2023 NVIDIA CORPORATION & AFFILIATES

  Licensed under the Apache License, Version 2.0 (the License);
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an AS IS BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.

----------------------------------------------------*/

package cdi_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cdiLib "github.com/container-orchestrated-devices/container-device-interface/pkg/cdi"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/cdi"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types/mocks"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/utils"
)

func TestCdi(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CDI Suite")
}

var _ = Describe("Cdi", func() {
	Context("successfully create CDI spec file", func() {
		It("should create CDI spec file", func() {
			staticDir := "cdi_static"
			dynamicDir := "cdi_dynamic"
			fs := &utils.FakeFilesystem{
				Dirs: []string{staticDir, dynamicDir},
			}
			defer fs.Use()()
			cdiLib.GetRegistry(cdiLib.WithSpecDirs(fs.RootDir+"/"+staticDir, fs.RootDir+"/"+dynamicDir))

			device := mocks.NewMockPciNetDevice(GinkgoT())
			device.On("GetPciAddr").Return("0000:00:00.1")
			device.On("GetRdmaSpec").Return([]*pluginapi.DeviceSpec{
				{HostPath: "host_path", ContainerPath: "container_path"},
			})

			err := cdi.New().CreateCDISpec("test-prefix", "test-name", "test-pool", []types.PciNetDevice{device})
			Expect(err).NotTo(HaveOccurred())

			cdiSpec, err := os.ReadFile(fs.RootDir + "/" + dynamicDir + "/test-prefix_test-pool.yaml")
			Expect(err).NotTo(HaveOccurred())

			substring := `
devices:
- containerEdits:
    deviceNodes:
    - hostPath: host_path
      path: container_path
      permissions: rw
  name: "0000:00:00.1"
kind: test-prefix/test-name`
			Expect(string(cdiSpec)).To(ContainSubstring(substring))
		})
	})
	Context("successfully create container annotation", func() {
		It("should return container annotation", func() {
			device := mocks.NewMockPciNetDevice(GinkgoT())
			device.On("GetPciAddr").Return("0000:00:00.1")
			cdi := cdi.New()
			annotations, err := cdi.CreateContainerAnnotations([]types.PciNetDevice{device}, "example.com", "net")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(annotations)).To(Equal(1))
			annoKey := "cdi.k8s.io/example.com_net"
			annoVal := "example.com/net=0000:00:00.1"
			Expect(annotations[annoKey]).To(Equal(annoVal))
		})
	})
})
