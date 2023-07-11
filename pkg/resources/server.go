package resources

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"golang.org/x/net/context"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	registerapi "k8s.io/kubelet/pkg/apis/pluginregistration/v1"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
)

const (
	// Local use
	cDialTimeout  = 5 * time.Second
	watchWaitTime = 5 * time.Second
)

type resourcesServerPort struct {
	server *grpc.Server
}

type resourceServer struct {
	resourceName   string
	watchMode      bool
	socketName     string
	socketPath     string
	stop           chan interface{}
	stopWatcher    chan bool
	updateResource chan bool
	health         chan *pluginapi.Device
	rsConnector    types.ResourceServerPort
	rdmaHcaMax     int
	// Mutex protects devs and deviceSpec
	mutex      sync.RWMutex
	devs       []*pluginapi.Device
	deviceSpec []*pluginapi.DeviceSpec
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
		c, err = grpc.DialContext(ctx, unixSocketPath, grpc.WithTransportCredentials(insecure.NewCredentials()),
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
func newResourceServer(config *types.UserConfig, devices []types.PciNetDevice, watcherMode bool,
	socketSuffix string) (types.ResourceServer, error) {
	var devs []*pluginapi.Device

	sockDir := activeSockDir

	if config.RdmaHcaMax < 0 {
		return nil, fmt.Errorf("error: Invalid value for rdmaHcaMax < 0: %d", config.RdmaHcaMax)
	}
	if config.ResourcePrefix == "" {
		return nil, fmt.Errorf("error: Empty resourcePrefix")
	}

	deviceSpec := getDevicesSpec(devices)

	if len(deviceSpec) > 0 {
		for n := 0; n < config.RdmaHcaMax; n++ {
			id := n
			dpDevice := &pluginapi.Device{
				ID:     strconv.Itoa(id),
				Health: pluginapi.Healthy,
			}
			devs = append(devs, dpDevice)
		}
	} else {
		log.Printf("Warning: no Rdma Devices were found for resource %s\n", config.ResourceName)
	}

	if !watcherMode {
		sockDir = deprecatedSockDir
	}

	socketName := fmt.Sprintf("%s.%s", config.ResourceName, socketSuffix)

	return &resourceServer{
		resourceName:   fmt.Sprintf("%s/%s", config.ResourcePrefix, config.ResourceName),
		socketName:     socketName,
		socketPath:     filepath.Join(sockDir, socketName),
		watchMode:      watcherMode,
		devs:           devs,
		deviceSpec:     deviceSpec,
		stop:           make(chan interface{}),
		stopWatcher:    make(chan bool),
		updateResource: make(chan bool, 1),
		health:         make(chan *pluginapi.Device),
		rsConnector:    &resourcesServerPort{},
		rdmaHcaMax:     config.RdmaHcaMax,
	}, nil
}

func detectPluginWatchMode(sockDir string) bool {
	if _, err := os.Stat(sockDir); err != nil {
		return false
	}
	return true
}

// Start starts the gRPC server of the device plugin
func (rs *resourceServer) Start() error {
	_ = rs.cleanup()
	log.Printf("starting %s device plugin endpoint at: %s\n", rs.resourceName, rs.socketName)
	rs.rsConnector.CreateServer()
	sock, err := rs.rsConnector.Listen("unix", rs.socketPath)
	if err != nil {
		return err
	}

	if rs.watchMode {
		registerapi.RegisterRegistrationServer(rs.rsConnector.GetServer(), rs)
	}
	pluginapi.RegisterDevicePluginServer(rs.rsConnector.GetServer(), rs)

	rs.rsConnector.Serve(sock)

	// Wait for server to start by launching a blocking connection
	conn, err := rs.rsConnector.Dial(rs.socketPath, cDialTimeout)
	if err != nil {
		return err
	}
	rs.rsConnector.Close(conn)

	log.Printf("%s device plugin endpoint started serving", rs.resourceName)

	if !rs.watchMode {
		if err = rs.register(); err != nil {
			rs.rsConnector.Stop()
			return err
		}
	}

	return nil
}

// Stop stops the gRPC server
func (rs *resourceServer) Stop() error {
	log.Printf("stopping %s device plugin server...", rs.resourceName)
	if rs.rsConnector == nil || rs.rsConnector.GetServer() == nil {
		return nil
	}

	// Send terminate signal to ListAndWatch()
	rs.stop <- true
	if !rs.watchMode {
		rs.stopWatcher <- true
	}

	rs.rsConnector.Stop()
	rs.rsConnector.DeleteServer()

	return rs.cleanup()
}

// Restart restart plugin server
func (rs *resourceServer) Restart() error {
	log.Printf("restarting %s device plugin server...", rs.resourceName)
	if rs.rsConnector == nil || rs.rsConnector.GetServer() == nil {
		return fmt.Errorf("grpc server instance not found for %s", rs.resourceName)
	}

	rs.rsConnector.Stop()
	rs.rsConnector.DeleteServer()

	// Send terminate signal to ListAndWatch()
	rs.stop <- true

	return rs.Start()
}

// Watch for Kubelet socket file; if not present restart server
func (rs *resourceServer) Watch() {
	// Watch for Kubelet socket file; if not present restart server
	for {
		select {
		case stop := <-rs.stopWatcher:
			if stop {
				log.Printf("kubelet watcher stopped for server %s", rs.socketPath)
				return
			}
		default:
			_, err := os.Lstat(rs.socketPath)
			if err != nil {
				// Socket file not found; restart server
				log.Printf("warning: server endpoint not found %s", rs.socketName)
				log.Printf("warning: most likely Kubelet restarted")
				if err := rs.Restart(); err != nil {
					log.Printf("error: unable to restart server %v", err)
				}
			}
		}
		// Sleep for some intervals; TODO: investigate on suggested interval
		time.Sleep(watchWaitTime)
	}
}

// Register registers the device plugin for the given resourceName with Kubelet.
func (rs *resourceServer) register() error {
	kubeletEndpoint := filepath.Join(deprecatedSockDir, kubeEndPoint)
	conn, err := rs.rsConnector.Dial(kubeletEndpoint, cDialTimeout)
	if err != nil {
		return err
	}
	defer rs.rsConnector.Close(conn)

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     rs.socketName,
		ResourceName: rs.resourceName,
	}

	return rs.rsConnector.Register(client, reqt)
}

// ListAndWatch lists devices and update that list according to the health status
func (rs *resourceServer) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	resp := new(pluginapi.ListAndWatchResponse)

	// Send initial list of devices
	if err := rs.sendDevices(resp, s); err != nil {
		return err
	}

	for {
		select {
		case <-s.Context().Done():
			log.Printf("ListAndWatch stream close: %v", s.Context().Err())
			return nil
		case <-rs.stop:
			return nil
		case d := <-rs.health:
			// FIXME: there is no way to recover from the Unhealthy state.
			d.Health = pluginapi.Unhealthy
			_ = s.Send(&pluginapi.ListAndWatchResponse{Devices: rs.devs})
		case <-rs.updateResource:
			if err := rs.sendDevices(resp, s); err != nil {
				// The old stream may not be closed properly, return to close it
				// and pass the update event to the new stream for processing
				rs.updateResource <- true
				return err
			}
		}
	}
}

func (rs *resourceServer) sendDevices(resp *pluginapi.ListAndWatchResponse,
	s pluginapi.DevicePlugin_ListAndWatchServer) error {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()
	log.Printf("Updating \"%s\" devices", rs.resourceName)
	resp.Devices = rs.devs

	if err := s.Send(resp); err != nil {
		log.Printf("error: failed to update \"%s\" resouces: %v", rs.resourceName, err)
		return err
	}
	log.Printf("exposing \"%d\" devices", len(rs.devs))
	return nil
}

// Allocate which return list of devices.
func (rs *resourceServer) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (
	*pluginapi.AllocateResponse, error) {
	log.Println("allocate request:", r)

	rs.mutex.RLock()
	defer rs.mutex.RUnlock()
	ress := make([]*pluginapi.ContainerAllocateResponse, len(r.GetContainerRequests()))

	for i := range r.GetContainerRequests() {
		ress[i] = &pluginapi.ContainerAllocateResponse{
			Devices: rs.deviceSpec,
		}
	}

	response := pluginapi.AllocateResponse{
		ContainerResponses: ress,
	}

	log.Println("allocate response: ", response)
	return &response, nil
}

// GetDevicePluginOptions returns options to be communicated with Device Manager
func (rs *resourceServer) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (
	*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{
		PreStartRequired: false,
	}, nil
}

// PreStartContainer is called, if indicated by Device Plugin during registeration phase
func (rs *resourceServer) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (
	*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (rs *resourceServer) cleanup() error {
	if err := os.Remove(rs.socketPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// GetInfo get info of plugin
func (rs *resourceServer) GetInfo(ctx context.Context, rqt *registerapi.InfoRequest) (*registerapi.PluginInfo, error) {
	pluginInfoResponse := &registerapi.PluginInfo{
		Type:              registerapi.DevicePlugin,
		Name:              rs.resourceName,
		Endpoint:          filepath.Join(activeSockDir, rs.socketName),
		SupportedVersions: []string{"v1alpha1", "v1beta1"},
	}
	return pluginInfoResponse, nil
}

// NotifyRegistrationStatus notify for registration status
func (rs *resourceServer) NotifyRegistrationStatus(ctx context.Context, regstat *registerapi.RegistrationStatus) (
	*registerapi.RegistrationStatusResponse, error) {
	if regstat.PluginRegistered {
		log.Printf("%s gets registered successfully at Kubelet \n", rs.socketName)
	} else {
		log.Printf("%s failed to be registered at Kubelet: %v; restarting.\n", rs.socketName, regstat.Error)
		rs.rsConnector.Stop()
	}
	return &registerapi.RegistrationStatusResponse{}, nil
}

func (rs *resourceServer) UpdateDevices(devices []types.PciNetDevice) {
	var needUpdate bool

	// Lock reading for plugin server for updating
	rs.mutex.Lock()
	defer func() {
		rs.mutex.Unlock()
		// Update event may block, so it must be sent after mutex.Unlock() to avoid deadlock caused by nesting
		if needUpdate {
			rs.updateResource <- true
		}
	}()

	// Get device spec
	deviceSpec := getDevicesSpec(devices)

	// If not devices not changed skip
	if !devicesChanged(rs.deviceSpec, deviceSpec) {
		log.Printf("no changes to devices for \"%s\"", rs.resourceName)
		log.Printf("exposing \"%d\" devices", len(rs.devs))
		return
	}

	rs.deviceSpec = deviceSpec

	// In case no RDMA resource report 0 resources
	if len(rs.deviceSpec) == 0 {
		rs.devs = []*pluginapi.Device{}
		needUpdate = true

		return
	}

	// Create devices list if not exists
	if len(rs.devs) == 0 {
		var devs []*pluginapi.Device
		for n := 0; n < rs.rdmaHcaMax; n++ {
			id := n
			dpDevice := &pluginapi.Device{
				ID:     strconv.Itoa(id),
				Health: pluginapi.Healthy,
			}
			devs = append(devs, dpDevice)
		}
		rs.devs = devs
	}

	needUpdate = true
}

func (rs *resourceServer) GetPreferredAllocation(
	ctx context.Context, req *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	return nil, nil
}

// devicesChanged detect if original and new devices are different
func devicesChanged(deviceList, newDeviceList []*pluginapi.DeviceSpec) bool {
	if len(deviceList) != len(newDeviceList) {
		return true
	}

	deviceListMap := map[string]bool{}
	for _, dev := range deviceList {
		deviceListMap[dev.HostPath] = true
	}

	for _, dev := range newDeviceList {
		if _, exists := deviceListMap[dev.HostPath]; !exists {
			return true
		}
	}

	return false
}

// getDevicesSpec return devicesSpec for given NetDevs
func getDevicesSpec(devices []types.PciNetDevice) []*pluginapi.DeviceSpec {
	devicesSpec := make([]*pluginapi.DeviceSpec, 0)
	for _, device := range devices {
		rdmaDeviceSpec := device.GetRdmaSpec()
		if len(rdmaDeviceSpec) == 0 {
			log.Printf("Warning: non-Rdma Device %s\n", device.GetPciAddr())
		}
		devicesSpec = append(devicesSpec, rdmaDeviceSpec...)
	}

	return devicesSpec
}
