package resources

import (
	"fmt"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/utils"
	"github.com/fsnotify/fsnotify"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	registerapi "k8s.io/kubernetes/pkg/kubelet/apis/pluginregistration/v1"
)

// resourceServer implements the Kubernetes device plugin API
type resourceServer struct {
	resourceName string
	socketName   string
	socketPath   string
	watchMode    bool
	devs         []*pluginapi.Device
	deviceSpec   []*pluginapi.DeviceSpec
	stop         chan interface{}
	stopWatcher  chan bool
	health       chan *pluginapi.Device
	server       *grpc.Server
}

// NewRdmaSharedDevPlugin returns an initialized resourceServer
func newResourceServer(config *types.UserConfig, watcherMode bool, resourcePrefix, socketSuffix string) (types.ResourceServer, error) {

	var devs []*pluginapi.Device
	deviceSpec := make([]*pluginapi.DeviceSpec, 0)

	sockDir := activeSockDir

	if config.RdmaHcaMax < 0 {
		return nil, fmt.Errorf("error: Invalid value for rdmaHcaMax < 0: %d", config.RdmaHcaMax)
	}

	for n := 0; n < config.RdmaHcaMax; n++ {
		id := n
		dpDevice := &pluginapi.Device{
			ID:     strconv.Itoa(id),
			Health: pluginapi.Healthy,
		}
		devs = append(devs, dpDevice)
	}

	for _, device := range config.Devices {
		pciAddress, err := utils.GetPciAddress(device)
		// Skip non existing devices
		if err != nil {
			continue
		}
		rdmaDeviceSpec := getRdmaDeviceSpec(pciAddress)
		if len(rdmaDeviceSpec) == 0 {
			log.Printf("Warning: non-Rdma Device %s\n", device)
		}
		deviceSpec = append(deviceSpec, rdmaDeviceSpec...)
	}

	if !watcherMode {
		sockDir = deprecatedSockDir
	}

	socketName := fmt.Sprintf("%s.%s", config.ResourceName, socketSuffix)

	return &resourceServer{
		resourceName: fmt.Sprintf("%s/%s", resourcePrefix, config.ResourceName),
		socketName:   socketName,
		socketPath:   filepath.Join(sockDir, socketName),
		watchMode:    watcherMode,
		devs:         devs,
		deviceSpec:   deviceSpec,
		stop:         make(chan interface{}),
		stopWatcher:  make(chan bool),
		health:       make(chan *pluginapi.Device),
	}, nil
}

func detectPluginWatchMode(sockDir string) bool {
	if _, err := os.Stat(sockDir); err != nil {
		return false
	}
	return true
}

// dial establishes the gRPC communication with the registered device plugin.
func dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	c, err := grpc.Dial(unixSocketPath, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(timeout),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		return nil, err
	}

	return c, nil
}

func getRdmaDeviceSpec(pciAddress string) []*pluginapi.DeviceSpec {
	deviceSpec := make([]*pluginapi.DeviceSpec, 0)

	rdmaDevices := utils.GetRdmaDevices(pciAddress)
	for _, device := range rdmaDevices {
		deviceSpec = append(deviceSpec, &pluginapi.DeviceSpec{
			HostPath:      device,
			ContainerPath: device,
			Permissions:   "rwm"})
	}

	return deviceSpec
}

// Start starts the gRPC server of the device plugin
func (m *resourceServer) Start() error {
	err := m.cleanup()
	if err != nil {
		return err
	}

	sock, err := net.Listen("unix", m.socketPath)
	if err != nil {
		return err
	}

	m.server = grpc.NewServer([]grpc.ServerOption{}...)

	if m.watchMode {
		registerapi.RegisterRegistrationServer(m.server, m)
	}
	pluginapi.RegisterDevicePluginServer(m.server, m)

	go m.server.Serve(sock)

	// Wait for server to start by launching a blocking connexion
	conn, err := dial(m.socketPath, 5*time.Second)
	if err != nil {
		return err
	}
	conn.Close()

	if !m.watchMode {
		if err = m.register(); err != nil {
			m.server.Stop()
			log.Fatal(err)
			return err
		}
	}

	// go m.healthcheck()

	return nil
}

// Stop stops the gRPC server
func (m *resourceServer) Stop() error {
	if m.server == nil {
		return nil
	}

	m.server.Stop()
	m.server = nil
	if !m.watchMode {
		m.stopWatcher <- true
	}
	close(m.stop)

	return m.cleanup()
}

// Restart restart plugin server
func (m *resourceServer) Restart() error {
	if err := m.Stop(); err != nil {
		return err
	}
	return m.Start()
}

// Watch watch for changes in old socket if used
func (m *resourceServer) Watch() {
	log.Println("Starting FS watcher.")
	watcher, err := newFSWatcher(deprecatedSockDir)
	if err != nil {
		log.Println("Failed to created FS watcher.")
		os.Exit(1)
	}
	defer watcher.Close()

	select {
	case event := <-watcher.Events:
		if event.Name == m.socketPath && event.Op&fsnotify.Create == fsnotify.Create {
			log.Printf("inotify: %s created, restarting.", m.socketPath)
			if err = m.Restart(); err != nil {
				log.Fatalf("unable to restart server %v", err)
			}
		}

	case err := <-watcher.Errors:
		log.Printf("inotify: %s", err)

	case stop := <-m.stopWatcher:
		if stop {
			log.Println("kubelet watcher stopped")
			watcher.Close()
			return
		}
	}
}

// Register registers the device plugin for the given resourceName with Kubelet.
func (m *resourceServer) register() error {
	kubeletEndpoint := filepath.Join(deprecatedSockDir, kubeEndPoint)
	conn, err := dial(kubeletEndpoint, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     m.socketName,
		ResourceName: m.resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}
	return nil
}

// ListAndWatch lists devices and update that list according to the health status
func (m *resourceServer) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	fmt.Println("exposing devices: ", m.devs)
	s.Send(&pluginapi.ListAndWatchResponse{Devices: m.devs})

	for {
		select {
		case <-m.stop:
			return nil
		case d := <-m.health:
			// FIXME: there is no way to recover from the Unhealthy state.
			d.Health = pluginapi.Unhealthy
			s.Send(&pluginapi.ListAndWatchResponse{Devices: m.devs})
		}
	}
}

func (m *resourceServer) unhealthy(dev *pluginapi.Device) {
	m.health <- dev
}

// Allocate which return list of devices.
func (m *resourceServer) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	log.Println("allocate request:", r)

	ress := make([]*pluginapi.ContainerAllocateResponse, len(r.GetContainerRequests()))

	for i := range r.GetContainerRequests() {
		ress[i] = &pluginapi.ContainerAllocateResponse{
			Devices: m.deviceSpec,
		}
	}

	response := pluginapi.AllocateResponse{
		ContainerResponses: ress,
	}

	log.Println("allocate response: ", response)
	return &response, nil
}

// GetDevicePluginOptions returns options to be communicated with Device Manager
func (m *resourceServer) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{
		PreStartRequired: false,
	}, nil
}

// PreStartContainer is called, if indicated by Device Plugin during registeration phase
func (m *resourceServer) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (m *resourceServer) cleanup() error {
	if err := os.Remove(m.socketPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// GetInfo get info of plugin
func (m *resourceServer) GetInfo(ctx context.Context, rqt *registerapi.InfoRequest) (*registerapi.PluginInfo, error) {
	pluginInfoResponse := &registerapi.PluginInfo{
		Type:              registerapi.DevicePlugin,
		Name:              m.resourceName,
		Endpoint:          m.socketPath,
		SupportedVersions: []string{"v1alpha1", "v1beta1"},
	}
	return pluginInfoResponse, nil
}

// NotifyRegistrationStatus notify for registration status
func (m *resourceServer) NotifyRegistrationStatus(ctx context.Context, regstat *registerapi.RegistrationStatus) (*registerapi.RegistrationStatusResponse, error) {
	if regstat.PluginRegistered {
		log.Printf("%s gets registered successfully at Kubelet \n", m.socketName)
	} else {
		log.Printf("%s failed to be registered at Kubelet: %v; restarting.\n", m.socketName, regstat.Error)
		m.server.Stop()
	}
	return &registerapi.RegistrationStatusResponse{}, nil
}
