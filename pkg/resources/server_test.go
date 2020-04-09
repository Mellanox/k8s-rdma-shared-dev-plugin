package resources

import (
	"context"
	"errors"
	"path"
	"time"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types/mocks"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/utils"

	"github.com/fsnotify/fsnotify"
	. "github.com/onsi/ginkgo"
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
	Context("newResourcesServer", func() {
		It("server with plugin watcher enabled", func() {
			fs := utils.FakeFilesystem{
				Dirs:     []string{fakeNetDevicePath},
				Symlinks: map[string]string{path.Join(fakeNetDevicePath, "device"): "../../../0000:02:00.0"},
			}
			defer fs.Use()()
			conf := &types.UserConfig{ResourceName: "test_server", RdmaHcaMax: 100, Devices: []string{"ib0"}}
			obj, err := newResourceServer(conf, true, "rdma", "socket")
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
			conf := &types.UserConfig{ResourceName: "test_server", RdmaHcaMax: 0, Devices: []string{"ib0"}}
			obj, err := newResourceServer(conf, true, "rdma", "socket")
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
			conf := &types.UserConfig{ResourceName: "test_server", RdmaHcaMax: 100, Devices: []string{"ib0"}}
			obj, err := newResourceServer(conf, false, "rdma", "socket")
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
			conf := &types.UserConfig{ResourceName: "test_server", RdmaHcaMax: 0, Devices: []string{"ib0"}}
			obj, err := newResourceServer(conf, false, "rdma", "socket")
			Expect(err).ToNot(HaveOccurred())
			rs := obj.(*resourceServer)
			Expect(rs.resourceName).To(Equal("rdma/test_server"))
			Expect(rs.socketName).To(Equal("test_server.socket"))
			Expect(rs.watchMode).To(Equal(false))
			Expect(len(rs.devs)).To(Equal(0))
		})
		It("server with plugin with invalid max number of resources", func() {
			conf := &types.UserConfig{ResourceName: "test_server", RdmaHcaMax: -100, Devices: []string{"ib0"}}
			obj, err := newResourceServer(conf, true, "rdma", "socket")
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
				shouldStop := <-stopWatcher
				Expect(shouldStop).To(BeTrue())
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

			err := rs.Restart()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("failed in restart"))
			rsc.AssertExpectations(testCallsAssertionReporter)
		})
		It("Failed to restart server", func() {
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
				stop:        make(chan interface{}),
			}

			err := rs.Restart()
			Expect(err).ToNot(HaveOccurred())
			rsc.AssertExpectations(testCallsAssertionReporter)
		})
	})
	Context("Watch", func() {
		var deprecatedSockDirBackup string
		var fs utils.FakeFilesystem
		var cleanTemp func()
		BeforeSuite(func() {
			deprecatedSockDirBackup = deprecatedSockDir
		})
		AfterSuite(func() {
			deprecatedSockDir = deprecatedSockDirBackup
		})
		BeforeEach(func() {
			fs = utils.FakeFilesystem{}
			cleanTemp = fs.Use()
			deprecatedSockDir = fs.RootDir
		})
		AfterEach(func() {
			cleanTemp()
		})
		It("Watch socket and send notification of create successfully", func() {
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
				socketPath:  "fake",
				stop:        make(chan interface{}),
			}
			go func() {
				// Wait for watch to create the socket watcher
				time.Sleep(5 * time.Millisecond)
				event := fsnotify.Event{Name: "fake", Op: fsnotify.Create}
				rs.socketWatcher.Events <- event
			}()
			err := rs.Watch()
			Expect(err).ToNot(HaveOccurred())
			rsc.AssertExpectations(testCallsAssertionReporter)
		})
		It("Watch socket and send notification of create and failed to restart server", func() {
			rsc := &mocks.ResourceServerConnector{}
			rsc.On("GetServer", mock.Anything).Return(nil)
			rsc.On("CreateServer").Return()
			rsc.On("Listen", mock.Anything, mock.Anything).Return(nil, errors.New("failed"))

			rs := resourceServer{
				watchMode:   true,
				socketPath:  "fake",
				rsConnector: rsc,
				stop:        make(chan interface{}),
			}
			go func() {
				// Wait for watch to create the socket watcher
				time.Sleep(5 * time.Millisecond)
				event := fsnotify.Event{Name: "fake", Op: fsnotify.Create}
				rs.socketWatcher.Events <- event
			}()
			err := rs.Watch()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("unable to restart server failed"))
			rsc.AssertExpectations(testCallsAssertionReporter)
		})
		It("Watch socket and getting failed error from watcher", func() {
			rs := resourceServer{}
			go func() {
				// Wait for watch to create the socket watcher
				time.Sleep(5 * time.Millisecond)
				err := errors.New("failed")
				rs.socketWatcher.Errors <- err
			}()
			err := rs.Watch()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("inotify: failed"))
		})
		It("Watch socket then stop watcher", func() {
			rs := resourceServer{stopWatcher: make(chan bool)}
			go func() {
				// Wait for watch to create the socket watcher
				time.Sleep(5 * time.Millisecond)
				rs.stopWatcher <- true
			}()
			err := rs.Watch()
			Expect(err).ToNot(HaveOccurred())
		})
	})
	Context("ListAndWatch", func() {
		It("Get devices of plugin and change device status", func() {
			obj, err := newResourceServer(&types.UserConfig{RdmaHcaMax: 100, ResourceName: "fake"}, true, "fake", "fake")
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
})
