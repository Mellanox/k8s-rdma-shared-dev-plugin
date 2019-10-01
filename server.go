package main

import (
	"fmt"
	"github.com/Mellanox/rdmamap"
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

// NewRdmaSharedDevPlugin returns an initialized RdmaDevPlugin
func NewRdmaSharedDevPlugin(config *UserConfig, watcherMode bool, resourcePrefix, socketSuffix string) (*RdmaDevPlugin, error) {

	var devs = []*pluginapi.Device{}
	deviceSpec := make([]*pluginapi.DeviceSpec, 0)
	var sockDir string

	log.Println("shared hca mode")

	if config.RdmaHcaMax < 0 {
		return nil, fmt.Errorf("Error: Invalid value for rdmaHcaMax < 0: %d", config.RdmaHcaMax)
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
		pciAddres, err := getPciAddress(device)
		// Skip non existing devices
		if err != nil {
			continue
		}
		rdmaDeviceSpec := getRdmaDeviceSpec(pciAddres)
		if len(rdmaDeviceSpec) == 0 {
			log.Printf("Warning: non-Rdma Device %s\n", device)
		}
		deviceSpec = append(deviceSpec, rdmaDeviceSpec...)
	}

	if watcherMode {
		sockDir = SockDir
	} else {
		sockDir = DeprecatedSockDir
	}

	socketName := fmt.Sprintf("%s.%s", config.ResourceName, socketSuffix)

	return &RdmaDevPlugin{
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
	rdmaResources := rdmamap.GetRdmaDevicesForPcidev(pciAddress)
	for _, resource := range rdmaResources {
		resRdmaDevices := rdmamap.GetRdmaCharDevices(resource)
		for _, rdmaDevice := range resRdmaDevices {
			deviceSpec = append(deviceSpec, &pluginapi.DeviceSpec{
				HostPath:      rdmaDevice,
				ContainerPath: rdmaDevice,
				Permissions:   "rwm",
			})
		}
	}

	return deviceSpec
}

func getPciAddress(ifName string) (string, error) {
	var pciAddress string
	ifaceDir := fmt.Sprintf("/sys/class/net/%s/device", ifName)
	dirInfo, err := os.Lstat(ifaceDir)
	if err != nil {
		return pciAddress, fmt.Errorf("can't get the symbolic link of the device %q: %v", ifName, err)
	}

	if (dirInfo.Mode() & os.ModeSymlink) == 0 {
		return pciAddress, fmt.Errorf("no symbolic link for the device %q", ifName)
	}

	pciInfo, err := os.Readlink(ifaceDir)
	if err != nil {
		return pciAddress, fmt.Errorf("can't read the symbolic link of the device %q: %v", ifName, err)
	}

	pciAddress = pciInfo[9:]
	return pciAddress, nil
}

// Start starts the gRPC server of the device plugin
func (m *RdmaDevPlugin) Start() error {
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
func (m *RdmaDevPlugin) Stop() error {
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

func (m *RdmaDevPlugin) Restart() error {
	if err := m.Stop(); err != nil {
		return err
	}
	return m.Start()
}

func (m *RdmaDevPlugin) Watch() {
	log.Println("Starting FS watcher.")
	watcher, err := newFSWatcher(DeprecatedSockDir)
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
func (m *RdmaDevPlugin) register() error {
	kubeletEndpoint := filepath.Join(DeprecatedSockDir, KubeEndPoint)
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
func (m *RdmaDevPlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
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

func (m *RdmaDevPlugin) unhealthy(dev *pluginapi.Device) {
	m.health <- dev
}

// Allocate which return list of devices.
func (m *RdmaDevPlugin) Allocate(ctx context.Context, r *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	log.Println("allocate request:", r)

	ress := make([]*pluginapi.ContainerAllocateResponse, len(r.GetContainerRequests()))

	for i, _ := range r.GetContainerRequests() {
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

func (m *RdmaDevPlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{
		PreStartRequired: false,
	}, nil
}

func (m *RdmaDevPlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (m *RdmaDevPlugin) cleanup() error {
	if err := os.Remove(m.socketPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (m *RdmaDevPlugin) GetInfo(ctx context.Context, rqt *registerapi.InfoRequest) (*registerapi.PluginInfo, error) {
	pluginInfoResponse := &registerapi.PluginInfo{
		Type:              registerapi.DevicePlugin,
		Name:              m.resourceName,
		Endpoint:          filepath.Join(SockDir, m.socketName),
		SupportedVersions: []string{"v1alpha1", "v1beta1"},
	}
	return pluginInfoResponse, nil
}

func (m *RdmaDevPlugin) NotifyRegistrationStatus(ctx context.Context, regstat *registerapi.RegistrationStatus) (*registerapi.RegistrationStatusResponse, error) {
	if regstat.PluginRegistered {
		log.Printf("%s gets registered successfully at Kubelet \n", m.socketName)
	} else {
		log.Printf("%s failed to be registered at Kubelet: %v; restarting.\n", m.socketName, regstat.Error)
		m.server.Stop()
	}
	return &registerapi.RegistrationStatusResponse{}, nil
}
