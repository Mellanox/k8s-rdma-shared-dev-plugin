package resources

import (
	"context"
	"errors"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types/mocks"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/utils"
	"github.com/fsnotify/fsnotify"

	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	registerapi "k8s.io/kubernetes/pkg/kubelet/apis/pluginregistration/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	"time"
)

type devPluginListAndWatchServerMock struct {
	grpc.ServerStream
}

func (x *devPluginListAndWatchServerMock) Send(m *pluginapi.ListAndWatchResponse) error {
	return nil
}

var _ = Describe("Server", func() {
	Context("newResourcesServer", func() {
		It("server with plugin watcher enabled", func() {
			fs := utils.FakeFilesystem{
				Dirs:     []string{"sys/class/net/ib0/"},
				Symlinks: map[string]string{"sys/class/net/ib0/device": "../../../0000:02:00.0"},
			}
			defer fs.Use()()
			conf := &types.UserConfig{ResourceName: "test_server", RdmaHcaMax: 100, Devices: []string{"ib0"}}
			obj := newResourceServer(conf, true, "rdma", "socket")
			rs := obj.(*resourceServer)
			Expect(rs.resourceName).To(Equal("rdma/test_server"))
			Expect(rs.endPoint).To(Equal("test_server.socket"))
			Expect(rs.watchMode).To(Equal(true))
			Expect(len(rs.devs)).To(Equal(100))
		})
		It("server with plugin watcher disabled", func() {
			fs := utils.FakeFilesystem{
				Dirs:     []string{"sys/class/net/ib0/"},
				Symlinks: map[string]string{"sys/class/net/ib0/device": "../../../0000:02:00.0"},
			}
			defer fs.Use()()
			conf := &types.UserConfig{ResourceName: "test_server", RdmaHcaMax: 100, Devices: []string{"ib0"}}
			obj := newResourceServer(conf, false, "rdma", "socket")
			rs := obj.(*resourceServer)
			Expect(rs.resourceName).To(Equal("rdma/test_server"))
			Expect(rs.endPoint).To(Equal("test_server.socket"))
			Expect(rs.watchMode).To(Equal(false))
			Expect(len(rs.devs)).To(Equal(100))
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
		})
		It("start server with failing to listen to socket", func() {
			rsc := &mocks.ResourceServerConnector{}
			rsc.On("CreateServer").Return()
			rsc.On("Listen", mock.Anything, mock.Anything).Return(nil, errors.New("failed"))
			rs := resourceServer{rsConnector: rsc}
			err := rs.Start()
			Expect(err).To(HaveOccurred())
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
		})
	})
	Context("Stop", func() {
		It("stop server with correct parameters", func() {
			grpcServer := grpc.NewServer([]grpc.ServerOption{}...)
			rsc := &mocks.ResourceServerConnector{}
			rsc.On("DeleteServer").Return()
			rsc.On("GetServer").Return(grpcServer)
			rsc.On("Stop").Return()

			stopWatcher := make(chan bool)
			rs := resourceServer{
				rsConnector: rsc,
				stopWatcher: stopWatcher,
				stop:        make(chan interface{}),
			}
			// Dummy listener to stopWatcher to not block the test and fail
			go func() {
				<-stopWatcher
			}()

			err := rs.Stop()
			Expect(err).ToNot(HaveOccurred())
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
		})
	})
	Context("Watch", func() {
		var DepSock string
		var fs utils.FakeFilesystem
		var cleanTemp func()
		BeforeSuite(func() {
			DepSock = deprecatedSockDir
		})
		AfterSuite(func() {
			deprecatedSockDir = DepSock
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
		It("GetInfo of plugin", func() {
			rs := resourceServer{
				stop:   make(chan interface{}),
				health: make(chan *pluginapi.Device),
			}
			// Dummy sender
			go func() {
				rs.health <- &pluginapi.Device{}
				// Make sure that health call before the stop
				time.Sleep(1 * time.Millisecond)
				rs.stop <- "stop"
			}()
			err := rs.ListAndWatch(nil, &devPluginListAndWatchServerMock{})
			Expect(err).ToNot(HaveOccurred())
		})
	})
	Context("Allocate", func() {
		It("Allocate resource", func() {
			rs := resourceServer{resourceName: "fake", endPoint: "fake.sock"}
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
			rs := resourceServer{resourceName: "fake", endPoint: "fake.sock"}
			resp, err := rs.GetInfo(context.TODO(), nil)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.Type).To(Equal(registerapi.DevicePlugin))
			Expect(resp.Name).To(Equal("fake"))
			Expect(resp.Endpoint).To(Equal("/var/lib/kubelet/plugins_registry/fake.sock"))
		})
	})
	Context("NotifyRegistrationStatus", func() {
		It("NotifyRegistrationStatus with plugin registered", func() {
			regstat := &registerapi.RegistrationStatus{PluginRegistered: true}
			rs := resourceServer{endPoint: "fake.sock"}
			_, err := rs.NotifyRegistrationStatus(context.TODO(), regstat)
			Expect(err).ToNot(HaveOccurred())
		})
		It("NotifyRegistrationStatus with plugin unregistered", func() {
			regstat := &registerapi.RegistrationStatus{PluginRegistered: false}
			rsc := &mocks.ResourceServerConnector{}
			rsc.On("Stop").Return()
			rs := resourceServer{endPoint: "fake.sock", rsConnector: rsc}
			_, err := rs.NotifyRegistrationStatus(context.TODO(), regstat)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
