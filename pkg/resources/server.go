package resources

import (
	"fmt"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/utils"
	"github.com/fsnotify/fsnotify"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	registerapi "k8s.io/kubernetes/pkg/kubelet/apis/pluginregistration/v1"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type resourcesServerPort struct {
	server *grpc.Server
}

type resourceServer struct {
	resourceName  string
	watchMode     bool
	socketName    string
	socketPath    string
	devs          []*pluginapi.Device
	deviceSpec    []*pluginapi.DeviceSpec
	stop          chan interface{}
	stopWatcher   chan bool
	health        chan *pluginapi.Device
	socketWatcher *fsnotify.Watcher
	rsConnector   types.ResourceServerPort
}

func (rsc *resourcesServerPort) GetServer() *grpc.Server {
	return rsc.server
}

func (rsc *resourcesServerPort) CreateServer() {
	rsc.server = grpc.NewServer([]grpc.ServerOption{}...)
}

func (rsc *resourcesServerPort) DeleteServer() {
	rsc.server = nil
}

func (rsc *resourcesServerPort) Listen(socketType, socketPath string) (net.Listener, error) {
	return net.Listen(socketType, socketPath)
}

func (rsc *resourcesServerPort) Serve(socket net.Listener) {
	go func() {
		_ = rsc.server.Serve(socket)
	}()
}

func (rsc *resourcesServerPort) Stop() {
	rsc.server.Stop()
}

func (rsc *resourcesServerPort) Close(clientConnection *grpc.ClientConn) {
	_ = clientConnection.Close()
}

func (rsc *resourcesServerPort) Register(client pluginapi.RegistrationClient, reqt *pluginapi.RegisterRequest) error {
	_, err := client.Register(context.Background(), reqt)
	return err
}

func (rsc *resourcesServerPort) Dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	var c *grpc.ClientConn
	var err error
	connChannel := make(chan interface{})

	ctx, timeoutCancel := context.WithTimeout(context.TODO(), timeout)
	defer timeoutCancel()
	go func() {
		c, err = grpc.DialContext(ctx, unixSocketPath, grpc.WithInsecure(),
			grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
				return net.Dial("unix", addr)
			}),
		)
		connChannel <- "done"
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("timout while trying to connect %s", unixSocketPath)

	case <-connChannel:
		return c, err
	}
}

// newResourceServer returns an initialized server
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
		rsConnector:  &resourcesServerPort{},
	}, nil
}

func detectPluginWatchMode(sockDir string) bool {
	if _, err := os.Stat(sockDir); err != nil {
		return false
	}
	return true
}

// Start starts the gRPC server of the device plugin
func (m *resourceServer) Start() error {
	err := m.cleanup()
	if err != nil {
		return err
	}

	m.rsConnector.CreateServer()
	sock, err := m.rsConnector.Listen("unix", m.socketPath)
	if err != nil {
		return err
	}

	if m.watchMode {
		registerapi.RegisterRegistrationServer(m.rsConnector.GetServer(), m)
	}
	pluginapi.RegisterDevicePluginServer(m.rsConnector.GetServer(), m)

	m.rsConnector.Serve(sock)

	// Wait for server to start by launching a blocking connexion
	conn, err := m.rsConnector.Dial(m.socketPath, 5*time.Second)
	if err != nil {
		return err
	}
	m.rsConnector.Close(conn)

	if !m.watchMode {
		if err = m.register(); err != nil {
			m.rsConnector.Stop()
			return err
		}
	}

	return nil
}

// Stop stops the gRPC server
func (m *resourceServer) Stop() error {
	if m.rsConnector == nil || m.rsConnector.GetServer() == nil {
		return nil
	}

	m.rsConnector.Stop()
	m.rsConnector.DeleteServer()
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
func (m *resourceServer) Watch() error {
	log.Println("Starting FS watcher.")
	watcher, err := newFSWatcher(deprecatedSockDir)
	if err != nil {
		log.Fatal("Failed to created FS watcher.")
	}
	m.socketWatcher = watcher
	defer watcher.Close()

	select {
	case event := <-watcher.Events:
		if event.Name == m.socketPath && event.Op&fsnotify.Create == fsnotify.Create {
			log.Printf("inotify: %s created, restarting.", m.socketPath)
			if err = m.Restart(); err != nil {
				return fmt.Errorf("unable to restart server %v", err)
			}
		}

	case err := <-watcher.Errors:
		return fmt.Errorf("inotify: %s", err)

	case stop := <-m.stopWatcher:
		if stop {
			log.Println("kubelet watcher stopped")
			_ = watcher.Close()
		}
	}
	return nil
}

// Register registers the device plugin for the given resourceName with Kubelet.
func (m *resourceServer) register() error {
	kubeletEndpoint := filepath.Join(deprecatedSockDir, kubeEndPoint)
	conn, err := m.rsConnector.Dial(kubeletEndpoint, 5*time.Second)
	if err != nil {
		return err
	}
	defer m.rsConnector.Close(conn)

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     m.socketName,
		ResourceName: m.resourceName,
	}

	return m.rsConnector.Register(client, reqt)
}

// ListAndWatch lists devices and update that list according to the health status
func (m *resourceServer) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	log.Println("exposing devices: ", m.devs)
	_ = s.Send(&pluginapi.ListAndWatchResponse{Devices: m.devs})

	for {
		select {
		case <-m.stop:
			return nil
		case d := <-m.health:
			// FIXME: there is no way to recover from the Unhealthy state.
			d.Health = pluginapi.Unhealthy
			_ = s.Send(&pluginapi.ListAndWatchResponse{Devices: m.devs})
		}
	}
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
		Endpoint:          filepath.Join(activeSockDir, m.socketName),
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
		m.rsConnector.Stop()
	}
	return &registerapi.RegistrationStatusResponse{}, nil
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
