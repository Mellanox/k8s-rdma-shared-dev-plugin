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
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cdiLib "github.com/container-orchestrated-devices/container-device-interface/pkg/cdi"
	cdiSpecs "github.com/container-orchestrated-devices/container-device-interface/specs-go"
	oci "github.com/opencontainers/runtime-spec/specs-go"
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

			err := cdi.New().CreateCDISpec("test-prefix", "test-name", "test-pool", []types.PciNetDevice{device}, nil)
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
  name: test-pool_0000:00:00.1
kind: test-prefix/test-name`
			Expect(string(cdiSpec)).To(ContainSubstring(substring))
		})
	})
	Context("device selected by more than one resource pool", func() {
		It("should keep the device resolvable for every pool", func() {
			fs := &utils.FakeFilesystem{
				Dirs: []string{"cdi_static", "cdi_dynamic"},
			}
			defer fs.Use()()
			registry := cdiLib.GetRegistry()
			previousDirs := registry.GetSpecDirectories()
			Expect(registry.Configure(cdiLib.WithSpecDirs(fs.RootDir+"/cdi_static", fs.RootDir+"/cdi_dynamic"),
				cdiLib.WithAutoRefresh(false))).To(Succeed())
			defer func() {
				Expect(registry.Configure(cdiLib.WithSpecDirs(previousDirs...), cdiLib.WithAutoRefresh(true))).To(Succeed())
			}()

			device := mocks.NewMockPciNetDevice(GinkgoT())
			device.On("GetPciAddr").Return("0000:00:00.1")
			device.On("GetRdmaSpec").Return([]*pluginapi.DeviceSpec{
				{HostPath: "/dev/infiniband/uverbs0", ContainerPath: "/dev/infiniband/uverbs0"},
			})
			devices := []types.PciNetDevice{device}
			pools := []string{"rdma_pool_a", "rdma_pool_b"}

			claimed := make(map[string]bool)
			for _, pool := range pools {
				legacy := cdi.ClaimDeviceAliases(claimed, "", devices)
				Expect(cdi.New().CreateCDISpec("nvidia.com", "net-rdma", pool, devices, legacy)).To(Succeed())
			}
			Expect(registry.Refresh()).To(Succeed())
			Expect(registry.GetErrors()).To(BeEmpty())

			// A container runtime resolves the devices named in the
			// annotations returned by Allocate.
			for _, pool := range pools {
				annotations, err := cdi.New().CreateContainerAnnotations(devices, "nvidia.com", "net-rdma", pool, pool)
				Expect(err).NotTo(HaveOccurred())
				_, cdiDevices, err := cdiLib.ParseAnnotations(annotations)
				Expect(err).NotTo(HaveOccurred())
				Expect(cdiDevices).To(HaveLen(1))

				Expect(registry.DeviceDB().GetDevice(cdiDevices[0])).NotTo(BeNil())
			}
		})
	})
	Context("upgrade from PCI-only device names", func() {
		var (
			registry    cdiLib.Registry
			previousDir []string
			specDirs    []string
		)
		BeforeEach(func() {
			root, err := os.MkdirTemp("", "cdi-upgrade-")
			Expect(err).NotTo(HaveOccurred())
			specDirs = []string{filepath.Join(root, "static"), filepath.Join(root, "dynamic")}
			for _, dir := range specDirs {
				Expect(os.Mkdir(dir, 0o755)).To(Succeed())
			}
			registry = cdiLib.GetRegistry()
			previousDir = registry.GetSpecDirectories()
			// Rescan the directories on every Refresh instead of waiting for
			// file system notifications.
			Expect(registry.Configure(cdiLib.WithSpecDirs(specDirs...), cdiLib.WithAutoRefresh(false))).To(Succeed())
		})
		AfterEach(func() {
			Expect(registry.Configure(cdiLib.WithSpecDirs(previousDir...), cdiLib.WithAutoRefresh(true))).To(Succeed())
			Expect(os.RemoveAll(filepath.Dir(specDirs[0]))).To(Succeed())
		})

		newDevice := func(pciAddr, hostPath string) types.PciNetDevice {
			device := mocks.NewMockPciNetDevice(GinkgoT())
			device.On("GetPciAddr").Return(pciAddr).Maybe()
			device.On("GetRdmaSpec").Return([]*pluginapi.DeviceSpec{
				{HostPath: hostPath, ContainerPath: "/dev/infiniband/uverbs0"},
			}).Maybe()
			return device
		}
		inject := func(annotations map[string]string) (*oci.Spec, error) {
			_, devices, err := cdiLib.ParseAnnotations(annotations)
			Expect(err).NotTo(HaveOccurred())
			Expect(devices).NotTo(BeEmpty())
			ociSpec := &oci.Spec{}
			_, err = registry.InjectDevices(ociSpec, devices...)
			return ociSpec, err
		}

		It("keeps annotations saved before the upgrade resolvable", func() {
			// Before the upgrade, rdma_pool_a selects 0000:02:00.0 and its spec
			// names the device by PCI address only.
			Expect(registry.SpecDB().WriteSpec(&cdiSpecs.Spec{
				Version: cdiSpecs.CurrentVersion,
				Kind:    "nvidia.com/net-rdma",
				Devices: []cdiSpecs.Device{{
					Name: "0000:02:00.0",
					ContainerEdits: cdiSpecs.ContainerEdits{DeviceNodes: []*cdiSpecs.DeviceNode{
						{Path: "/dev/infiniband/uverbs0", HostPath: "/dev/null", Permissions: "rw"},
					}},
				}},
			}, "nvidia.com_rdma_pool_a")).To(Succeed())
			// Kubelet saved the annotations returned by Allocate for this pool.
			saved := map[string]string{
				"cdi.k8s.io/rdmadp_IMWEESHCY7UUQJYWCZD54VDBKL2BGSHIOVPVAY7REY3THNLTTIHQ": "nvidia.com/net-rdma=0000:02:00.0",
			}
			Expect(registry.Refresh()).To(Succeed())
			_, err := inject(saved)
			Expect(err).NotTo(HaveOccurred())

			// The upgraded plugin removes the old specs and writes new ones. The
			// new configuration also adds rdma_pool_b, which selects the same NIC
			// and one more.
			Expect(cdi.CleanupSpecs("nvidia.com", "net-rdma")).To(Succeed())
			device1 := newDevice("0000:02:00.0", "/dev/null")
			device2 := newDevice("0000:03:00.0", "/dev/zero")
			pools := []struct {
				name    string
				devices []types.PciNetDevice
			}{
				{"rdma_pool_a", []types.PciNetDevice{device1}},
				{"rdma_pool_b", []types.PciNetDevice{device1, device2}},
			}
			claimed := make(map[string]bool)
			for _, pool := range pools {
				legacy := cdi.ClaimDeviceAliases(claimed, "", pool.devices)
				Expect(cdi.New().CreateCDISpec("nvidia.com", "net-rdma", pool.name, pool.devices, legacy)).To(Succeed())
			}
			Expect(registry.Refresh()).To(Succeed())
			Expect(registry.GetErrors()).To(BeEmpty())

			// A container of the existing Pod restarts with the saved annotations.
			ociSpec, err := inject(saved)
			Expect(err).NotTo(HaveOccurred())
			Expect(ociSpec.Linux.Devices).To(HaveLen(1))
			Expect(ociSpec.Linux.Devices[0].Path).To(Equal("/dev/infiniband/uverbs0"))

			// Each legacy name is defined once, by the first pool that selects the device.
			for name, specFile := range map[string]string{
				"nvidia.com/net-rdma=0000:02:00.0": "nvidia.com_rdma_pool_a.yaml",
				"nvidia.com/net-rdma=0000:03:00.0": "nvidia.com_rdma_pool_b.yaml",
			} {
				device := registry.DeviceDB().GetDevice(name)
				Expect(device).NotTo(BeNil(), name)
				Expect(filepath.Base(device.GetSpec().GetPath())).To(Equal(specFile))
			}

			// New allocations use the pool-prefixed names.
			for _, pool := range pools {
				annotations, err := cdi.New().CreateContainerAnnotations(
					pool.devices, "nvidia.com", "net-rdma", pool.name, pool.name)
				Expect(err).NotTo(HaveOccurred())
				_, err = inject(annotations)
				Expect(err).NotTo(HaveOccurred())
			}
		})
	})
	Context("successfully create container annotation", func() {
		It("should return container annotation", func() {
			device := mocks.NewMockPciNetDevice(GinkgoT())
			device.On("GetPciAddr").Return("0000:00:00.1")
			cdi := cdi.New()
			annotations, err := cdi.CreateContainerAnnotations(
				[]types.PciNetDevice{device}, "example.com", "net", "pool", "pool")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(annotations)).To(Equal(1))
			annoVal := "example.com/net=pool_0000:00:00.1"
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
				[]types.PciNetDevice{device1}, "nvidia.com", "net-rdma", "rdma_pool_a", "rdma_pool_a")
			Expect(err).NotTo(HaveOccurred())
			annotations2, err := cdi.CreateContainerAnnotations(
				[]types.PciNetDevice{device2}, "nvidia.com", "net-rdma", "rdma_pool_b", "rdma_pool_b")
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
				"nvidia.com/net-rdma=rdma_pool_a_0000:00:00.1",
				"nvidia.com/net-rdma=rdma_pool_b_0000:00:00.2",
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
			first, err := cdi.CreateContainerAnnotations([]types.PciNetDevice{device1}, prefix, "net-rdma", name, name)
			Expect(err).NotTo(HaveOccurred())
			second, err := cdi.CreateContainerAnnotations([]types.PciNetDevice{device2}, prefix, "net-rdma", name, name)
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
			_, err := cdi.New().CreateContainerAnnotations([]types.PciNetDevice{device}, "nvidia.com", "net-rdma", "", "")
			Expect(err).To(HaveOccurred())
		})
	})
})
