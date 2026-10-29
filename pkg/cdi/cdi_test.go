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
	"strings"
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
			annotations, err := cdi.CreateContainerAnnotations([]types.PciNetDevice{device}, "example.com", "net", "pool")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(annotations)).To(Equal(1))
			annoVal := "example.com/net=0000:00:00.1"
			for key, val := range annotations {
				Expect(key).To(MatchRegexp(`^cdi\.k8s\.io/rdmadp_[A-Z2-7]{52}$`))
				Expect(val).To(Equal(annoVal))
			}
		})
		It("should return distinct annotation keys for different pools", func() {
			device1 := mocks.NewMockPciNetDevice(GinkgoT())
			device1.On("GetPciAddr").Return("0000:00:00.1")
			device2 := mocks.NewMockPciNetDevice(GinkgoT())
			device2.On("GetPciAddr").Return("0000:00:00.2")
			cdi := cdi.New()
			annotations1, err := cdi.CreateContainerAnnotations(
				[]types.PciNetDevice{device1}, "nvidia.com", "net-rdma", "rdma_pool_a")
			Expect(err).NotTo(HaveOccurred())
			annotations2, err := cdi.CreateContainerAnnotations(
				[]types.PciNetDevice{device2}, "nvidia.com", "net-rdma", "rdma_pool_b")
			Expect(err).NotTo(HaveOccurred())

			merged := map[string]string{}
			for k, v := range annotations1 {
				merged[k] = v
			}
			for k, v := range annotations2 {
				merged[k] = v
			}
			Expect(merged).To(HaveLen(2))
			Expect(merged).To(ContainElements(
				"nvidia.com/net-rdma=0000:00:00.1",
				"nvidia.com/net-rdma=0000:00:00.2",
			))
		})
		It("supports long resource names with stable, valid annotation keys", func() {
			device1 := mocks.NewMockPciNetDevice(GinkgoT())
			device1.On("GetPciAddr").Return("0000:00:00.1")
			device2 := mocks.NewMockPciNetDevice(GinkgoT())
			device2.On("GetPciAddr").Return("0000:00:00.2")
			cdi := cdi.New()
			prefix := strings.Repeat("a", 63) + ".example.com"
			name := strings.Repeat("b", 63) + "_"
			first, err := cdi.CreateContainerAnnotations([]types.PciNetDevice{device1}, prefix, "net-rdma", name)
			Expect(err).NotTo(HaveOccurred())
			second, err := cdi.CreateContainerAnnotations([]types.PciNetDevice{device2}, prefix, "net-rdma", name)
			Expect(err).NotTo(HaveOccurred())
			Expect(first).To(HaveLen(1))
			Expect(second).To(HaveLen(1))
			for key := range first {
				Expect(key).To(MatchRegexp(`^cdi\.k8s\.io/rdmadp_[A-Z2-7]{52}$`))
				Expect(second).To(HaveKey(key))
			}
		})
		It("should fail with empty pool name", func() {
			device := mocks.NewMockPciNetDevice(GinkgoT())
			_, err := cdi.New().CreateContainerAnnotations([]types.PciNetDevice{device}, "nvidia.com", "net-rdma", "")
			Expect(err).To(HaveOccurred())
		})
	})
})
