package resources

import (
	"context"
	"encoding/json"
	"errors"
	"path"
	"time"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types/mocks"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	registerapi "k8s.io/kubelet/pkg/apis/pluginregistration/v1"
)

const (
	fakeNetDevicePath = "sys/class/net/ib0/"
)

type devPluginListAndWatchServerMock struct {
	grpc.ServerStream
	devices []*pluginapi.Device
}

func (x *devPluginListAndWatchServerMock) Send(m *pluginapi.ListAndWatchResponse) error {
	x.devices = m.Devices
	return nil
}

var _ = Describe("resourceServer tests", func() {
	fakeDeviceSpec := []*pluginapi.DeviceSpec{{HostPath: "fake", ContainerPath: "fake"}}
	fakePciDevice := &mocks.PciNetDevice{}
	fakePciDevice.On("GetRdmaSpec").Return(fakeDeviceSpec)
	fakeDeviceList := []types.PciNetDevice{fakePciDevice}
	Context("newResourcesServer", func() {
		It("server with plugin watcher enabled", func() {
			fs := utils.FakeFilesystem{
				Dirs:     []string{fakeNetDevicePath},
				Symlinks: map[string]string{path.Join(fakeNetDevicePath, "device"): "../../../0000:02:00.0"},
			}
			defer fs.Use()()
			conf := &types.UserConfig{ResourceName: "test_server", RdmaHcaMax: 100}
			obj, err := newResourceServer(conf, fakeDeviceList, true, "rdma", "socket")
			Expect(err).ToNot(HaveOccurred())
			rs := obj.(*resourceServer)
			Expect(rs.resourceName).To(Equal("rdma/test_server"))
			Expect(rs.socketName).To(Equal("test_server.socket"))
			Expect(rs.watchMode).To(Equal(true))
			Expect(len(rs.devs)).To(Equal(100))
		})
		It("server with plugin watcher enabled with 0 resources", func() {
			fs := utils.FakeFilesystem{
				Dirs:     []string{fakeNetDevicePath},
				Symlinks: map[string]string{path.Join(fakeNetDevicePath, "device"): "../../../0000:02:00.0"},
			}
			defer fs.Use()()
			conf := &types.UserConfig{ResourceName: "test_server", RdmaHcaMax: 0}
			obj, err := newResourceServer(conf, fakeDeviceList, true, "rdma", "socket")
			Expect(err).ToNot(HaveOccurred())
			rs := obj.(*resourceServer)
			Expect(rs.resourceName).To(Equal("rdma/test_server"))
			Expect(rs.socketName).To(Equal("test_server.socket"))
			Expect(rs.watchMode).To(Equal(true))
			Expect(len(rs.devs)).To(Equal(0))
		})
		It("server with no RDMA resources", func() {
			fs := utils.FakeFilesystem{
				Dirs:     []string{fakeNetDevicePath},
				Symlinks: map[string]string{path.Join(fakeNetDevicePath, "device"): "../../../0000:02:00.0"},
			}
			defer fs.Use()()
			conf := &types.UserConfig{ResourceName: "test_server", RdmaHcaMax: 100}
			fakePciDevice := &mocks.PciNetDevice{}
			fakePciDevice.On("GetRdmaSpec").Return([]*pluginapi.DeviceSpec{})
			fakePciDevice.On("GetPciAddr").Return("0000:02:00.0")
			deviceList := []types.PciNetDevice{fakePciDevice}
			obj, err := newResourceServer(conf, deviceList, true, "rdma", "socket")
			Expect(err).ToNot(HaveOccurred())
			rs := obj.(*resourceServer)
			Expect(rs.resourceName).To(Equal("rdma/test_server"))
			Expect(rs.socketName).To(Equal("test_server.socket"))
			Expect(rs.watchMode).To(Equal(true))
			Expect(len(rs.devs)).To(Equal(0))
		})
		It("server with plugin watcher disabled", func() {
			fs := utils.FakeFilesystem{
				Dirs:     []string{fakeNetDevicePath},
				Symlinks: map[string]string{path.Join(fakeNetDevicePath, "device"): "../../../0000:02:00.0"},
			}
			defer fs.Use()()
			conf := &types.UserConfig{ResourceName: "test_server", RdmaHcaMax: 100}
			obj, err := newResourceServer(conf, fakeDeviceList, false, "rdma", "socket")
			Expect(err).ToNot(HaveOccurred())
			rs := obj.(*resourceServer)
			Expect(rs.resourceName).To(Equal("rdma/test_server"))
			Expect(rs.socketName).To(Equal("test_server.socket"))
			Expect(rs.watchMode).To(Equal(false))
			Expect(len(rs.devs)).To(Equal(100))
		})
		It("server with plugin watcher disabled with 0 resources", func() {
			fs := utils.FakeFilesystem{
				Dirs:     []string{fakeNetDevicePath},
				Symlinks: map[string]string{path.Join(fakeNetDevicePath, "device"): "../../../0000:02:00.0"},
			}
			defer fs.Use()()
			conf := &types.UserConfig{ResourceName: "test_server", RdmaHcaMax: 0}
			obj, err := newResourceServer(conf, fakeDeviceList, false, "rdma", "socket")
			Expect(err).ToNot(HaveOccurred())
			rs := obj.(*resourceServer)
			Expect(rs.resourceName).To(Equal("rdma/test_server"))
			Expect(rs.socketName).To(Equal("test_server.socket"))
			Expect(rs.watchMode).To(Equal(false))
			Expect(len(rs.devs)).To(Equal(0))
		})
		It("server with plugin with invalid max number of resources", func() {
			conf := &types.UserConfig{ResourceName: "test_server", RdmaHcaMax: -100}
			obj, err := newResourceServer(conf, fakeDeviceList, true, "rdma", "socket")
			Expect(err).To(HaveOccurred())
			Expect(obj).To(BeNil())
		})
	})
	Context("Start", func() {
		It("start server with plugin watcher enabled", func() {
			grpcServer := grpc.NewServer([]grpc.ServerOption{}...)
			rsc := &mocks.ResourceServerConnector{}
			rsc.On("GetServer", mock.Anything).Return(grpcServer)
			rsc.On("CreateServer").Return()
			rsc.On("Listen", mock.Anything, mock.Anything).Return(nil, nil)
			rsc.On("Serve", mock.Anything).Return()
			rsc.On("Dial", mock.Anything, mock.Anything).Return(nil, nil)
			rsc.On("Close", mock.Anything).Return()
			rs := resourceServer{watchMode: true, rsConnector: rsc}
			err := rs.Start()
			Expect(err).ToNot(HaveOccurred())
			rsc.AssertExpectations(testCallsAssertionReporter)
		})
		It("start server with plugin watcher disabled", func() {
			grpcServer := grpc.NewServer([]grpc.ServerOption{}...)
			rsc := &mocks.ResourceServerConnector{}
			rsc.On("GetServer", mock.Anything).Return(grpcServer)
			rsc.On("CreateServer").Return()
			rsc.On("Listen", mock.Anything, mock.Anything).Return(nil, nil)
			rsc.On("Serve", mock.Anything).Return()
			rsc.On("Dial", mock.Anything, mock.Anything).Return(nil, nil)
			rsc.On("Close", mock.Anything).Return()
			rsc.On("Register", mock.Anything, mock.Anything).Return(nil)
			rs := resourceServer{watchMode: false, rsConnector: rsc}
			err := rs.Start()
			Expect(err).ToNot(HaveOccurred())
			rsc.AssertExpectations(testCallsAssertionReporter)
		})
		It("start server with failing to listen to socket", func() {
			rsc := &mocks.ResourceServerConnector{}
			rsc.On("CreateServer").Return()
			rsc.On("Listen", mock.Anything, mock.Anything).Return(nil, errors.New("failed"))
			rs := resourceServer{rsConnector: rsc}
			err := rs.Start()
			Expect(err).To(HaveOccurred())
			rsc.AssertExpectations(testCallsAssertionReporter)
		})
		It("start server with plugin failed to dial server", func() {
			grpcServer := grpc.NewServer([]grpc.ServerOption{}...)
			rsc := &mocks.ResourceServerConnector{}
			rsc.On("GetServer", mock.Anything).Return(grpcServer)
			rsc.On("CreateServer").Return()
			rsc.On("Listen", mock.Anything, mock.Anything).Return(nil, nil)
			rsc.On("Serve", mock.Anything).Return()
			rsc.On("Dial", mock.Anything, mock.Anything).Return(nil, errors.New("failed"))
			rs := resourceServer{rsConnector: rsc}
			err := rs.Start()
			Expect(err).To(HaveOccurred())
			rsc.AssertExpectations(testCallsAssertionReporter)
		})
		It("start server with plugin watcher disabled failed to register", func() {
			grpcServer := grpc.NewServer([]grpc.ServerOption{}...)
			rsc := &mocks.ResourceServerConnector{}
			rsc.On("GetServer", mock.Anything).Return(grpcServer)
			rsc.On("CreateServer").Return()
			rsc.On("Listen", mock.Anything, mock.Anything).Return(nil, nil)
			rsc.On("Serve", mock.Anything).Return()
			rsc.On("Dial", mock.Anything, mock.Anything).Return(nil, nil)
			rsc.On("Close", mock.Anything).Return()
			rsc.On("Stop").Return()
			rsc.On("Register", mock.Anything, mock.Anything).Return(errors.New("failed"))
			rs := resourceServer{watchMode: false, rsConnector: rsc}
			err := rs.Start()
			Expect(err).To(HaveOccurred())
			rsc.AssertExpectations(testCallsAssertionReporter)
		})
	})
	Context("Stop", func() {
		It("stop server with correct parameters with watch mode enabled", func() {
			grpcServer := grpc.NewServer([]grpc.ServerOption{}...)
			rsc := &mocks.ResourceServerConnector{}
			rsc.On("DeleteServer").Return()
			rsc.On("GetServer").Return(grpcServer)
			rsc.On("Stop").Return()

			rs := resourceServer{
				rsConnector: rsc,
				watchMode:   true,
				stop:        make(chan interface{}),
			}

			go func() {
				stop := <-rs.stop
				Expect(stop).To(BeTrue())
			}()

			err := rs.Stop()
			Expect(err).ToNot(HaveOccurred())
			rsc.AssertExpectations(testCallsAssertionReporter)
		})
		It("stop server with correct parameters with watch mode disabled", func() {
			grpcServer := grpc.NewServer([]grpc.ServerOption{}...)
			rsc := &mocks.ResourceServerConnector{}
			rsc.On("DeleteServer").Return()
			rsc.On("GetServer").Return(grpcServer)
			rsc.On("Stop").Return()

			stopWatcher := make(chan bool)
			rs := resourceServer{
				rsConnector: rsc,
				watchMode:   false,
				stopWatcher: stopWatcher,
				stop:        make(chan interface{}),
			}
			// Dummy listener to stopWatcher to not block the test and fail
			go func() {
				stop := <-rs.stop
				Expect(stop).To(BeTrue())
				stop = <-rs.stopWatcher
				Expect(stop).To(BeTrue())
			}()

			err := rs.Stop()
			Expect(err).ToNot(HaveOccurred())
			rsc.AssertExpectations(testCallsAssertionReporter)
		})
		It("stop non existing server", func() {
			rs := resourceServer{}
			err := rs.Stop()
			Expect(err).ToNot(HaveOccurred())
		})
	})
	Context("Restart", func() {
		It("Restart server with correct parameters", func() {
			grpcServer := grpc.NewServer([]grpc.ServerOption{}...)
			rsc := &mocks.ResourceServerConnector{}
			rsc.On("GetServer", mock.Anything).Return(grpcServer)
			rsc.On("CreateServer").Return()
			rsc.On("DeleteServer").Return()
			rsc.On("Stop").Return()
			rsc.On("Listen", mock.Anything, mock.Anything).Return(nil, errors.New("failed in restart"))

			rs := resourceServer{
				watchMode:   true,
				rsConnector: rsc,
				stop:        make(chan interface{}),
			}

			go func() {
				stop := <-rs.stop
				Expect(stop).To(BeTrue())
			}()

			err := rs.Restart()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed in restart"))
			rsc.AssertExpectations(testCallsAssertionReporter)
		})
		It("Failed to restart server with no grpc server", func() {
			rs := resourceServer{
				watchMode: true,
				stop:      make(chan interface{}),
			}

			err := rs.Restart()
			Expect(err).To(HaveOccurred())
		})
		It("Failed to restart server", func() {
			grpcServer := grpc.NewServer([]grpc.ServerOption{}...)
			rsc := &mocks.ResourceServerConnector{}
			rsc.On("GetServer", mock.Anything).Return(grpcServer)
			rsc.On("Stop").Return()
			rsc.On("DeleteServer").Return()
			rsc.On("CreateServer").Return()
			rsc.On("Listen", mock.Anything, mock.Anything).Return(nil, nil)
			rsc.On("Serve", mock.Anything).Return()
			rsc.On("Dial", mock.Anything, mock.Anything).Return(nil, errors.New("error"))

			rs := resourceServer{
				watchMode:   true,
				rsConnector: rsc,
				stop:        make(chan interface{}),
			}

			go func() {
				stop := <-rs.stop
				Expect(stop).To(BeTrue())
			}()

			err := rs.Restart()
			Expect(err).To(HaveOccurred())
			rsc.AssertExpectations(testCallsAssertionReporter)
		})
	})
	Context("Watch", func() {
		fakeSocketName := "fake.socket"
		var fakeSocketPath string
		var fs utils.FakeFilesystem
		deprecatedSockDirBackup := deprecatedSockDir
		var cleanTemp func()
		BeforeEach(func() {
			fs = utils.FakeFilesystem{
				Files: map[string][]byte{fakeSocketName: []byte("")},
			}
			cleanTemp = fs.Use()
			deprecatedSockDir = fs.RootDir
			fakeSocketPath = path.Join(fs.RootDir, fakeSocketName)
		})
		AfterEach(func() {
			cleanTemp()
			deprecatedSockDir = deprecatedSockDirBackup
		})
		It("Watch socket then stop watcher", func() {
			rs := resourceServer{
				socketName:  fakeSocketName,
				socketPath:  fakeSocketPath,
				stopWatcher: make(chan bool),
				stop:        make(chan interface{}),
			}
			go func() {
				rs.stopWatcher <- true
			}()
			rs.Watch()
		})
		It("Watch socket and send notification to restart successfully", func() {
			grpcServer := grpc.NewServer([]grpc.ServerOption{}...)
			rsc := &mocks.ResourceServerConnector{}
			rsc.On("GetServer", mock.Anything).Return(grpcServer)
			rsc.On("CreateServer").Return()
			rsc.On("DeleteServer").Return()
			rsc.On("Listen", mock.Anything, mock.Anything).Return(nil, nil)
			rsc.On("Serve", mock.Anything).Return()
			rsc.On("Dial", mock.Anything, mock.Anything).Return(nil, nil)
			rsc.On("Stop").Return()
			rsc.On("Close", mock.Anything).Return()

			rs := resourceServer{
				watchMode:   true,
				rsConnector: rsc,
				socketName:  fakeSocketName,
				socketPath:  "fake deleted",
				stop:        make(chan interface{}),
				stopWatcher: make(chan bool),
			}
			go func() {
				stop := <-rs.stop
				Expect(stop).To(BeTrue())
				rs.stopWatcher <- true
			}()
			rs.Watch()
			rsc.AssertExpectations(testCallsAssertionReporter)
		})
	})
	Context("ListAndWatch", func() {
		It("Get devices of plugin and change device status", func() {
			fs := utils.FakeFilesystem{
				Dirs:     []string{fakeNetDevicePath},
				Symlinks: map[string]string{path.Join(fakeNetDevicePath, "device"): "../../../0000:02:00.0"},
			}
			defer fs.Use()()
			conf := &types.UserConfig{RdmaHcaMax: 100, ResourceName: "fake"}
			obj, err := newResourceServer(conf, fakeDeviceList, true, "fake", "fake")
			Expect(err).ToNot(HaveOccurred())

			rs := obj.(*resourceServer)

			rs.stop = make(chan interface{})
			rs.health = make(chan *pluginapi.Device)
			// Dummy sender
			go func() {
				rs.health <- rs.devs[5]
				// Make sure that health call before the stop
				time.Sleep(1 * time.Millisecond)
				rs.stop <- "stop"
			}()

			s := &devPluginListAndWatchServerMock{}
			err = rs.ListAndWatch(nil, s)
			Expect(err).ToNot(HaveOccurred())
			Expect(s.devices).To(Equal(rs.devs))
			Expect(len(s.devices)).To(Equal(100))
			Expect(s.devices[5].Health).To(Equal(pluginapi.Unhealthy))
		})
	})
	Context("Allocate", func() {
		It("Allocate resource", func() {
			rs := resourceServer{resourceName: "fake", socketName: "fake.sock"}
			req := &pluginapi.AllocateRequest{
				ContainerRequests: []*pluginapi.ContainerAllocateRequest{nil, nil},
			}
			res, err := rs.Allocate(context.TODO(), req)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(res.ContainerResponses)).To(Equal(2))
		})
	})
	Context("GetInfo", func() {
		It("GetInfo of plugin", func() {
			rs := resourceServer{resourceName: "fake", socketName: "fake.sock"}
			resp, err := rs.GetInfo(context.TODO(), nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.Type).To(Equal(registerapi.DevicePlugin))
			Expect(resp.Name).To(Equal("fake"))
			Expect(resp.Endpoint).To(Equal(path.Join(activeSockDir, "fake.sock")))
		})
	})
	Context("NotifyRegistrationStatus", func() {
		It("NotifyRegistrationStatus with plugin registered", func() {
			regstat := &registerapi.RegistrationStatus{PluginRegistered: true}
			rs := resourceServer{socketName: "fake.sock"}
			_, err := rs.NotifyRegistrationStatus(context.TODO(), regstat)
			Expect(err).ToNot(HaveOccurred())
		})
		It("NotifyRegistrationStatus with plugin unregistered", func() {
			regstat := &registerapi.RegistrationStatus{PluginRegistered: false}
			rsc := &mocks.ResourceServerConnector{}
			rsc.On("Stop").Return()
			rs := resourceServer{socketName: "fake.sock", rsConnector: rsc}
			_, err := rs.NotifyRegistrationStatus(context.TODO(), regstat)
			Expect(err).ToNot(HaveOccurred())
			rsc.AssertExpectations(testCallsAssertionReporter)
		})
	})
	Context("UpdateDevices", func() {
		It("should receive signal of updating resource", func() {
			rs := &resourceServer{
				updateResource: make(chan bool),
				rdmaHcaMax:     10,
			}

			go func() { rs.UpdateDevices(fakeDeviceList) }()
			Expect(<-rs.updateResource).To(BeTrue())
			Expect(len(rs.deviceSpec)).To(Equal(1))
			Expect(len(rs.devs)).To(Equal(10))
		})
		It("resources not updated", func() {
			rs := &resourceServer{
				updateResource: make(chan bool),
				rdmaHcaMax:     10,
			}

			var emptyDevicesList []types.PciNetDevice
			rs.UpdateDevices(emptyDevicesList)
			Expect(len(rs.deviceSpec)).To(Equal(0))
			Expect(len(rs.devs)).To(Equal(0))
		})
	})
	Context("devicesChanged", func() {
		It("device is present and did not change", func() {
			deviceList := []*pluginapi.DeviceSpec{{HostPath: "/foo/bar"}}
			newDeviceList := []*pluginapi.DeviceSpec{{HostPath: "/foo/bar"}}
			changed := devicesChanged(deviceList, newDeviceList)
			Expect(changed).To(BeFalse())
		})
		It("device changed - num of devices in deviceList", func() {
			deviceList := []*pluginapi.DeviceSpec{{HostPath: "/foo/bar"}}
			newDeviceList := []*pluginapi.DeviceSpec{{HostPath: "/foo/bar"}, {HostPath: "/foo/bar2"}}
			changed := devicesChanged(deviceList, newDeviceList)
			Expect(changed).To(BeTrue())
		})
		It("device changed - mounts changed for one of the devices in the deviceList", func() {
			deviceList := []*pluginapi.DeviceSpec{{HostPath: "/foo/bar"}}
			newDeviceList := []*pluginapi.DeviceSpec{{HostPath: "/foo/bar2"}}
			changed := devicesChanged(deviceList, newDeviceList)
			Expect(changed).To(BeTrue())
		})
	})
	DescribeTable("registering with Kubelet",
		func(shouldRunServer, shouldEnablePluginWatch, shouldServerFail, shouldFail bool) {
			fs := &utils.FakeFilesystem{}
			defer fs.Use()()

			// Use faked dir as socket dir
			activeSockDirBackup := activeSockDir
			deprecatedSockDirBackup := deprecatedSockDir

			deprecatedSockDir = fs.RootDir
			activeSockDir = fs.RootDir

			defer func() {
				deprecatedSockDir = deprecatedSockDirBackup
				activeSockDir = activeSockDirBackup
			}()

			conf := &types.UserConfig{ResourceName: "fake_test", RdmaHcaMax: 100}
			obj, err := newResourceServer(conf, fakeDeviceList, true, "rdma", "socket")
			Expect(err).ToNot(HaveOccurred())
			rs := obj.(*resourceServer)

			registrationServer := createFakeRegistrationServer(deprecatedSockDir,
				"fake_test.socket", shouldServerFail, shouldEnablePluginWatch)

			if shouldRunServer {
				if shouldEnablePluginWatch {
					_ = rs.Start()
				} else {
					registrationServer.start()
				}
			}
			if shouldEnablePluginWatch {
				err = registrationServer.registerPlugin()
			} else {
				err = rs.register()
			}
			if shouldFail {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			if shouldRunServer {
				if shouldEnablePluginWatch {
					rs.rsConnector.Stop()
				} else {
					registrationServer.stop()
				}
			}
		},
		Entry("when can't connect to Kubelet should fail", false, false, true, true),
		Entry("when device plugin unable to register with Kubelet should fail", true, false, true, true),
		Entry("when Kubelet unable to register with device plugin should fail", true, true, true, true),
		Entry("successfully shouldn't fail", true, false, false, false),
		Entry("successfully shouldn't fail with plugin watcher enabled", true, true, false, false),
	)
	Describe("resource server lifecycle", func() {
		// integration-like test for the resource server (positive cases)
		var (
			fs                      *utils.FakeFilesystem
			activeSockDirBackup     string
			deprecatedSockDirBackup string
		)
		BeforeEach(func() {
			activeSockDirBackup = activeSockDir
			deprecatedSockDirBackup = deprecatedSockDir
			selectors := &types.Selectors{}
			err := json.Unmarshal([]byte(`{"deviceIDs": ["fakeid"]}`), selectors)
			Expect(err).NotTo(HaveOccurred())
			fs = &utils.FakeFilesystem{}
		})
		AfterEach(func() {
			activeSockDir = activeSockDirBackup
			deprecatedSockDir = deprecatedSockDirBackup
		})
		Context("starting, restarting and stopping the resource server", func() {
			It("should not fail and messages should be received on the channels without watcher mode", func() {
				defer fs.Use()()
				// Use faked dir as socket dir
				deprecatedSockDir = fs.RootDir

				conf := &types.UserConfig{ResourceName: "fakename", RdmaHcaMax: 100}
				obj, err := newResourceServer(conf, fakeDeviceList, false, "rdma", "socket")
				Expect(err).ToNot(HaveOccurred())
				rs := obj.(*resourceServer)

				registrationServer := createFakeRegistrationServer(deprecatedSockDir,
					"fakename.socket", false, false)
				registrationServer.start()
				defer registrationServer.stop()

				err = rs.Start()
				Expect(err).NotTo(HaveOccurred())

				go func() {
					stop := <-rs.stop
					Expect(stop).To(BeTrue())
				}()
				err = rs.Restart()
				Expect(err).NotTo(HaveOccurred())

				go func() {
					stop := <-rs.stop
					Expect(stop).To(BeTrue())
					stop = <-rs.stopWatcher
					Expect(stop).To(BeTrue())
				}()

				err = rs.Stop()
				Expect(err).NotTo(HaveOccurred())
			})
			It("should not fail and messages should be received on the channels with watcher mode", func() {
				defer fs.Use()()
				// Use faked dir as socket dir
				activeSockDir = fs.RootDir

				conf := &types.UserConfig{ResourceName: "fakename", RdmaHcaMax: 100}
				obj, err := newResourceServer(conf, fakeDeviceList, true, "rdma", "socket")
				Expect(err).ToNot(HaveOccurred())
				rs := obj.(*resourceServer)

				registrationServer := createFakeRegistrationServer(activeSockDir,
					"fakename.socket", false, true)

				err = rs.Start()
				Expect(err).NotTo(HaveOccurred())

				err = registrationServer.registerPlugin()
				Expect(err).NotTo(HaveOccurred())

				go func() {
					stop := <-rs.stop
					Expect(stop).To(BeTrue())
				}()
				err = rs.Restart()
				Expect(err).NotTo(HaveOccurred())

				go func() {
					stop := <-rs.stop
					Expect(stop).To(BeTrue())
				}()

				err = rs.Stop()
				Expect(err).NotTo(HaveOccurred())
			})
		})
		Context("starting, watching and stopping the resource server", func() {
			It("should not fail and messages should be received on the channels", func() {
				defer fs.Use()()
				// Use faked dir as socket dir
				deprecatedSockDir = fs.RootDir

				conf := &types.UserConfig{ResourceName: "fakename", RdmaHcaMax: 100}
				obj, err := newResourceServer(conf, fakeDeviceList, false, "rdma", "socket")
				Expect(err).ToNot(HaveOccurred())
				rs := obj.(*resourceServer)

				registrationServer := createFakeRegistrationServer(deprecatedSockDir,
					"fakename.socket", false, false)
				registrationServer.start()
				defer registrationServer.stop()

				err = rs.Start()
				Expect(err).NotTo(HaveOccurred())
				// run socket watcher in background as in real-life
				go rs.Watch()
				go func() {
					stop := <-rs.stop
					Expect(stop).To(BeTrue())
				}()
				err = rs.Stop()
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	DescribeTable("allocating",
		func(req *pluginapi.AllocateRequest, expectedRespLength int, shouldFail bool) {
			conf := &types.UserConfig{ResourceName: "fakename", RdmaHcaMax: 100}
			obj, err := newResourceServer(conf, fakeDeviceList, true, "rdma", "socket")
			Expect(err).ToNot(HaveOccurred())
			rs := obj.(*resourceServer)

			resp, err := rs.Allocate(context.TODO(), req)

			Expect(len(resp.GetContainerResponses())).To(Equal(expectedRespLength))

			if shouldFail {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		},
		Entry("allocating successfully 1 deviceID",
			&pluginapi.AllocateRequest{
				ContainerRequests: []*pluginapi.ContainerAllocateRequest{{DevicesIDs: []string{"00:00.01"}}},
			},
			1,
			false,
		),
		PEntry("allocating deviceID that does not exist",
			&pluginapi.AllocateRequest{
				ContainerRequests: []*pluginapi.ContainerAllocateRequest{{DevicesIDs: []string{"00:00.02"}}},
			},
			0,
			true,
		),
		Entry("empty AllocateRequest", &pluginapi.AllocateRequest{}, 0, false),
	)
})
