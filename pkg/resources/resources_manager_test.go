package resources

import (
	"errors"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types/mocks"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/utils"
	"path"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ResourcesManger", func() {
	Context("NewResourceManager", func() {
		It("Resources Manger with watcher mode", func() {
			fs := utils.FakeFilesystem{
				Dirs: []string{"var/lib/kubelet/plugins_registry"},
			}
			defer fs.Use()()
			activeSockDir = path.Join(fs.RootDir, "var/lib/kubelet/plugins_registry")
			defer func() {
				activeSockDir = "/var/lib/kubelet/plugins_registry"
			}()

			obj := NewResourceManager()
			rm := obj.(*resourceManager)
			Expect(rm.watchMode).To(Equal(true))
		})
		It("Resources Manger without watcher mode", func() {
			fs := utils.FakeFilesystem{}
			defer fs.Use()()
			activeSockDir = path.Join(fs.RootDir, "noDir")
			defer func() {
				activeSockDir = "/var/lib/kubelet/plugins_registry"
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
             "devices": ["ib3", "ib4"]
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
		It("Read valid config file", func() {
			configData := `{"configList": [{
             "resourceName": "hca_shared_devices_a",
             "rdmaHcaMax": 1000,
             "devices": ["ib0", "ib1"]
           },
           {
             "resourceName": "hca_shared_devices_b",
             "rdmaHcaMax": 500,
             "devices": ["ib3", "ib4"]
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
		})
	})
	Context("ValidateConfigs", func() {
		It("Valid config list", func() {
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
		It("resources with no devices", func() {
			var configlist []*types.UserConfig
			rm := &resourceManager{}

			configlist = append(configlist, &types.UserConfig{
				ResourceName: "test_config",
				RdmaHcaMax:   100})

			rm.configList = configlist
			err := rm.ValidateConfigs()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("error: no devices provided"))
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
		})
	})
	Context("StartAllServers", func() {
		It("start valid server", func() {
			fakeDevicePlugin := mocks.ResourceServer{}
			fakeDevicePlugin.On("Start").Return(nil)
			fakeDevicePlugin.On("Watch").Return(nil)

			rm := &resourceManager{resourceServers: []types.ResourceServer{&fakeDevicePlugin}}

			err := rm.StartAllServers()
			Expect(err).ToNot(HaveOccurred())
		})
		It("start invalid server", func() {
			fakeDevicePlugin := mocks.ResourceServer{}
			fakeDevicePlugin.On("Start").Return(errors.New("failed"))

			rm := &resourceManager{resourceServers: []types.ResourceServer{&fakeDevicePlugin}}

			err := rm.StartAllServers()
			Expect(err).To(HaveOccurred())
		})
	})
	Context("StopAllServers", func() {
		It("stop valid server", func() {
			fakeDevicePlugin := mocks.ResourceServer{}
			fakeDevicePlugin.On("Stop").Return(nil)

			rm := &resourceManager{resourceServers: []types.ResourceServer{&fakeDevicePlugin}}

			err := rm.StopAllServers()
			Expect(err).ToNot(HaveOccurred())
		})
		It("stop invalid server", func() {
			fakeDevicePlugin := mocks.ResourceServer{}
			fakeDevicePlugin.On("Stop").Return(errors.New("failed"))

			rm := &resourceManager{resourceServers: []types.ResourceServer{&fakeDevicePlugin}}

			err := rm.StopAllServers()
			Expect(err).To(HaveOccurred())
		})
	})
	Context("RestartAllServers", func() {
		It("restart valid server", func() {
			fakeDevicePlugin := mocks.ResourceServer{}
			fakeDevicePlugin.On("Restart").Return(nil)

			rm := &resourceManager{resourceServers: []types.ResourceServer{&fakeDevicePlugin}}

			err := rm.RestartAllServers()
			Expect(err).ToNot(HaveOccurred())
		})
		It("restart invalid server", func() {
			fakeDevicePlugin := mocks.ResourceServer{}
			fakeDevicePlugin.On("Restart").Return(errors.New("failed"))

			rm := &resourceManager{resourceServers: []types.ResourceServer{&fakeDevicePlugin}}

			err := rm.RestartAllServers()
			Expect(err).To(HaveOccurred())
		})
	})
})
