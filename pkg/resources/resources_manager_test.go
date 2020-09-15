package resources

import (
	"errors"
	"os"
	"path"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types/mocks"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/utils"

	"github.com/jaypipes/ghw"
	"github.com/jaypipes/pcidb"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourcesManger", func() {
	Context("NewResourceManager", func() {
		const activeSockDirBackUP = "/var/lib/kubelet/plugins_registry"

		It("Resource Manager with watcher mode", func() {
			fs := utils.FakeFilesystem{
				Dirs: []string{activeSockDir[1:]},
			}
			defer fs.Use()()
			activeSockDir = path.Join(fs.RootDir, activeSockDirBackUP[1:])
			defer func() {
				activeSockDir = activeSockDirBackUP
			}()

			obj := NewResourceManager()
			rm := obj.(*resourceManager)
			Expect(rm.watchMode).To(Equal(true))
		})
		It("Resource Manager without watcher mode", func() {
			fs := utils.FakeFilesystem{}
			defer fs.Use()()
			activeSockDir = path.Join(fs.RootDir, "noDir")
			defer func() {
				activeSockDir = activeSockDirBackUP
			}()

			obj := NewResourceManager()
			rm := obj.(*resourceManager)
			Expect(rm.watchMode).To(Equal(false))
		})
	})
	Context("ReadConfig", func() {
		It("Read valid config file", func() {
			configData := `{"configList": [{
             "resourceName": "hca_shared_devices_a",
             "rdmaHcaMax": 1000,
             "devices": ["ib0", "ib1"]
           },
           {
             "resourceName": "hca_shared_devices_b",
             "rdmaHcaMax": 500,
             "selectors": {"vendors": ["15b3"],
                           "ifNames": ["ib2", "ib3"]}
           }
        ]}`
			fs := &utils.FakeFilesystem{
				Dirs: []string{"tmp"},
				Files: map[string][]byte{
					"tmp/config.json": []byte(configData),
				},
			}
			defer fs.Use()()

			rm := &resourceManager{configFile: fs.RootDir + "/tmp/config.json"}
			err := rm.ReadConfig()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(rm.configList)).To(Equal(2))
			Expect(len(rm.configList[0].Devices)).To(Equal(2))
			Expect(len(rm.configList[1].Selectors.Vendors)).To(Equal(1))
			Expect(len(rm.configList[1].Selectors.IfNames)).To(Equal(2))
		})
		It("non existing config file", func() {

			rm := &resourceManager{configFile: "/tmp/config.json"}
			err := rm.ReadConfig()
			Expect(err).To(HaveOccurred())
		})
		It("Read invalid config file", func() {
			configData := `{"configList": [{
             "resourceName": "hca_shared_devices_a",
             "rdmaHcaMax": 1000,
             "devices": ["ib0", "ib1"]
           }}},
        ]}`
			fs := &utils.FakeFilesystem{
				Dirs: []string{"tmp"},
				Files: map[string][]byte{
					"tmp/config.json": []byte(configData),
				},
			}
			defer fs.Use()()

			rm := &resourceManager{configFile: fs.RootDir + "/tmp/config.json"}
			err := rm.ReadConfig()
			Expect(err).To(HaveOccurred())
		})
	})
	Context("ValidateConfigs", func() {
		It("Valid config list with \"devices\" field", func() {
			var configlist []*types.UserConfig
			rm := &resourceManager{}

			configlist = append(configlist, &types.UserConfig{
				ResourceName: "test_config",
				RdmaHcaMax:   100,
				Devices:      []string{"ib0"}})

			rm.configList = configlist
			err := rm.ValidateConfigs()
			Expect(err).ToNot(HaveOccurred())
		})
		It("Valid config list  \"selectors\" field", func() {
			var configlist []*types.UserConfig
			rm := &resourceManager{}

			configlist = append(configlist, &types.UserConfig{
				ResourceName: "test_config",
				RdmaHcaMax:   100,
				Selectors: types.Selectors{
					Vendors: []string{"15b3"},
					IfNames: []string{"eth1"}},
			})

			rm.configList = configlist
			err := rm.ValidateConfigs()
			Expect(err).ToNot(HaveOccurred())
		})
		It("Valid config list with 0 number of resources", func() {
			var configlist []*types.UserConfig
			rm := &resourceManager{}

			configlist = append(configlist, &types.UserConfig{
				ResourceName: "test_config",
				RdmaHcaMax:   0,
				Devices:      []string{"ib0"}})

			rm.configList = configlist
			err := rm.ValidateConfigs()
			Expect(err).ToNot(HaveOccurred())
		})
		It("Validate empty config list", func() {
			rm := &resourceManager{}
			err := rm.ValidateConfigs()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("no resources configuration found"))
		})
		It("resources with invalid name", func() {
			var configlist []*types.UserConfig
			rm := &resourceManager{}

			configlist = append(configlist, &types.UserConfig{
				ResourceName: "test_config$$",
				RdmaHcaMax:   100,
				Devices:      []string{"ib0"}})

			rm.configList = configlist
			err := rm.ValidateConfigs()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error: resource name \"test_config$$\" contains invalid characters"))
		})
		It("resources with repeated names", func() {
			var configlist []*types.UserConfig
			rm := &resourceManager{}

			configlist = append(configlist, &types.UserConfig{
				ResourceName: "test_config",
				RdmaHcaMax:   100,
				Devices:      []string{"ib0"}})
			configlist = append(configlist, &types.UserConfig{
				ResourceName: "test_config",
				RdmaHcaMax:   500,
				Devices:      []string{"ib1"}})

			rm.configList = configlist
			err := rm.ValidateConfigs()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error: resource name \"test_config\" already exists"))
		})
		It("resources with invalid number of resources", func() {
			var configlist []*types.UserConfig
			rm := &resourceManager{}

			configlist = append(configlist, &types.UserConfig{
				ResourceName: "test_config",
				RdmaHcaMax:   -100,
				Devices:      []string{"ib0"}})

			rm.configList = configlist
			err := rm.ValidateConfigs()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error: Invalid value for rdmaHcaMax < 0: -100"))
		})
		It("configuration mismatch between \"selectors\" and \"devices\"", func() {
			var configlist []*types.UserConfig
			rm := &resourceManager{}

			configlist = append(configlist, &types.UserConfig{
				ResourceName: "test_config",
				RdmaHcaMax:   100,
				Devices:      []string{"eth0"},
				Selectors:    types.Selectors{IfNames: []string{"eth1"}},
			})

			rm.configList = configlist
			err := rm.ValidateConfigs()
			Expect(err).To(HaveOccurred())
		})
		It("resources configuration with neither \"selectors\" nor \"devices\"", func() {
			var configlist []*types.UserConfig
			rm := &resourceManager{}

			configlist = append(configlist, &types.UserConfig{
				ResourceName: "test_config",
				RdmaHcaMax:   100,
			})

			rm.configList = configlist
			err := rm.ValidateConfigs()
			Expect(err).To(HaveOccurred())
		})
	})
	Context("GetDevices", func() {
		It("Get full list of devices", func() {
			deviceList := []*ghw.PCIDevice{
				{Address: "0000:02:00.0", Vendor: &pcidb.Vendor{ID: "15b3"}},
				{Address: "0000:03:00.0", Vendor: &pcidb.Vendor{ID: "8080"}}}
			rm := &resourceManager{deviceList: deviceList}
			Expect(len(rm.GetDevices())).To(Equal(2))
		})
	})
	Context("GetFilteredDevices", func() {
		It("Get full list of devices", func() {
			dev1 := &mocks.PciNetDevice{}
			dev2 := &mocks.PciNetDevice{}
			dev3 := &mocks.PciNetDevice{}
			dev4 := &mocks.PciNetDevice{}
			dev1.On("GetVendor").Return("15b3")
			dev1.On("GetIfName").Return("enp2s0f0")

			dev2.On("GetVendor").Return("8080")
			dev2.On("GetIfName").Return("enp2s0f1")

			dev3.On("GetVendor").Return("15b3")
			dev3.On("GetIfName").Return("eth0")

			dev4.On("GetVendor").Return("8080")
			dev4.On("GetIfName").Return("eth1")

			devices := []types.PciNetDevice{dev1, dev2, dev3, dev4}

			selectors := types.Selectors{
				Vendors: []string{"15b3", "8080"},
				IfNames: []string{"enp2s0f0", "enp2s0f1"},
			}
			rm := &resourceManager{}
			filteredDevices := rm.GetFilteredDevices(devices, selectors)

			Expect(len(filteredDevices)).To(Equal(2))
			Expect(filteredDevices[0]).To(Equal(dev1))
			Expect(filteredDevices[1]).To(Equal(dev2))
		})
	})
	Context("DiscoverHostDevices", func() {
		It("Discover devices in host", func() {
			fs := &utils.FakeFilesystem{
				Dirs: []string{
					"sys/bus/pci/devices/0000:02:00.0",
					"sys/bus/pci/devices/0000:08:00.0"},
				Files: map[string][]byte{
					"sys/bus/pci/devices/0000:02:00.0/modalias": []byte(
						"pci:v000015B3d00001017sv000015B3sd00000001bc02sc00i00"),
					"sys/bus/pci/devices/0000:08:00.0/modalias": []byte(
						"pci:v00008086d00001D02sv000015D9sd00000717bc01sc06i01")},
			}
			defer fs.Use()()
			os.Setenv("GHW_CHROOT", fs.RootDir)
			defer os.Unsetenv("GHW_CHROOT")

			rm := &resourceManager{}

			err := rm.DiscoverHostDevices()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(rm.deviceList)).To(Equal(1))
		})
		It("Discover zero devices in host", func() {
			fs := &utils.FakeFilesystem{}
			defer fs.Use()()
			os.Setenv("GHW_CHROOT", fs.RootDir)
			defer os.Unsetenv("GHW_CHROOT")

			rm := &resourceManager{}

			err := rm.DiscoverHostDevices()
			Expect(err).NotTo(HaveOccurred())
			Expect(len(rm.deviceList)).To(BeZero())
		})
	})
	Context("InitServers", func() {
		It("Init valid server", func() {
			var configlist []*types.UserConfig
			rm := &resourceManager{}

			configlist = append(configlist, &types.UserConfig{
				ResourceName: "test_config",
				RdmaHcaMax:   100,
				Devices:      []string{"ib0"}})

			rm.configList = configlist
			err := rm.InitServers()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(rm.configList)).To(Equal(1))
			Expect(len(rm.resourceServers)).To(Equal(1))
		})
		It("Init server with invalid number of resources", func() {
			var configlist []*types.UserConfig
			rm := &resourceManager{}

			configlist = append(configlist, &types.UserConfig{
				ResourceName: "test_config",
				RdmaHcaMax:   -100,
				Devices:      []string{"ib0"}})

			rm.configList = configlist
			err := rm.InitServers()
			Expect(err).To(HaveOccurred())
		})
	})
	Context("StartAllServers", func() {
		It("start valid server with watcher enabled", func() {
			fakeResourceServer := mocks.ResourceServer{}
			fakeResourceServer.On("Start").Return(nil)

			rm := &resourceManager{watchMode: true,
				resourceServers: []types.ResourceServer{&fakeResourceServer}}

			err := rm.StartAllServers()
			Expect(err).ToNot(HaveOccurred())
			fakeResourceServer.AssertExpectations(testCallsAssertionReporter)
		})
		It("start valid server with watcher disabled", func() {
			fakeResourceServer := mocks.ResourceServer{}
			fakeResourceServer.On("Start").Return(nil)
			fakeResourceServer.On("Watch").Return(nil)

			rm := &resourceManager{watchMode: false,
				resourceServers: []types.ResourceServer{&fakeResourceServer}}

			err := rm.StartAllServers()
			Expect(err).ToNot(HaveOccurred())
		})
		It("start invalid server", func() {
			fakeResourceServer := mocks.ResourceServer{}
			fakeResourceServer.On("Start").Return(errors.New("failed"))

			rm := &resourceManager{resourceServers: []types.ResourceServer{&fakeResourceServer}}

			err := rm.StartAllServers()
			Expect(err).To(HaveOccurred())
			fakeResourceServer.AssertExpectations(testCallsAssertionReporter)
		})
	})
	Context("StopAllServers", func() {
		It("stop valid server", func() {
			fakeResourceServer := mocks.ResourceServer{}
			fakeResourceServer.On("Stop").Return(nil)

			rm := &resourceManager{resourceServers: []types.ResourceServer{&fakeResourceServer}}
			// make sure that stop will be called
			Expect(len(rm.resourceServers)).To(BeNumerically(">", 0))

			err := rm.StopAllServers()
			Expect(err).ToNot(HaveOccurred())
			fakeResourceServer.AssertExpectations(testCallsAssertionReporter)
		})
		It("stop invalid server", func() {
			fakeResourceServer := mocks.ResourceServer{}
			fakeResourceServer.On("Stop").Return(errors.New("failed to stop"))

			rm := &resourceManager{resourceServers: []types.ResourceServer{&fakeResourceServer}}
			// make sure that stop will be called
			Expect(len(rm.resourceServers)).To(BeNumerically(">", 0))

			err := rm.StopAllServers()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed to stop"))
			fakeResourceServer.AssertExpectations(testCallsAssertionReporter)
		})
	})
	Context("RestartAllServers", func() {
		It("restart valid server", func() {
			fakeResourceServer := mocks.ResourceServer{}
			fakeResourceServer.On("Restart").Return(nil)

			rm := &resourceManager{resourceServers: []types.ResourceServer{&fakeResourceServer}}
			// make sure that Restart will be called
			Expect(len(rm.resourceServers)).To(BeNumerically(">", 0))

			err := rm.RestartAllServers()
			Expect(err).ToNot(HaveOccurred())
			fakeResourceServer.AssertExpectations(testCallsAssertionReporter)
		})
		It("restart invalid server", func() {
			fakeResourceServer := mocks.ResourceServer{}
			fakeResourceServer.On("Restart").Return(errors.New("failed to restart"))

			rm := &resourceManager{resourceServers: []types.ResourceServer{&fakeResourceServer}}
			// make sure that Restart will be called
			Expect(len(rm.resourceServers)).To(BeNumerically(">", 0))

			err := rm.RestartAllServers()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed to restart"))
			fakeResourceServer.AssertExpectations(testCallsAssertionReporter)
		})
	})
})
