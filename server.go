package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path"
	"strconv"
	"time"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	RdmaSharedDpSocket = "rdma-shared-dp.sock"
)

const (
	RdmaHcaResourceName = "rdma/hca"
)

const (
	RdmaDevices = "/dev/infiniband"
)

type UserConfig struct {
	RdmaHcaMax int `json:"rdmaHcaMax"`
}

// RdmaDevPlugin implements the Kubernetes device plugin API
type RdmaDevPlugin struct {
	resourceName string
	socket       string
	devs         []*pluginapi.Device

	stop   chan interface{}
	health chan *pluginapi.Device

	server *grpc.Server
}

// NewRdmaSharedDevPlugin returns an initialized RdmaDevPlugin
func NewRdmaSharedDevPlugin(config UserConfig) (*RdmaDevPlugin, error) {

	var devs = []*pluginapi.Device{}

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

	return &RdmaDevPlugin{
		resourceName: RdmaHcaResourceName,
		socket:       pluginapi.DevicePluginPath + RdmaSharedDpSocket,

		devs: devs,

		stop:   make(chan interface{}),
		health: make(chan *pluginapi.Device),
	}, nil
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

// Start starts the gRPC server of the device plugin
func (m *RdmaDevPlugin) Start() error {
	err := m.cleanup()
	if err != nil {
		return err
	}

	sock, err := net.Listen("unix", m.socket)
	if err != nil {
		return err
	}

	m.server = grpc.NewServer([]grpc.ServerOption{}...)
	pluginapi.RegisterDevicePluginServer(m.server, m)

	go m.server.Serve(sock)

	// Wait for server to start by launching a blocking connexion
	conn, err := dial(m.socket, 5*time.Second)
	if err != nil {
		return err
	}
	conn.Close()

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
	close(m.stop)

	return m.cleanup()
}

// Register registers the device plugin for the given resourceName with Kubelet.
func (m *RdmaDevPlugin) Register(kubeletEndpoint, resourceName string) error {
	conn, err := dial(kubeletEndpoint, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(m.socket),
		ResourceName: resourceName,
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
		ds := make([]*pluginapi.DeviceSpec, 1)
		ds[0] = &pluginapi.DeviceSpec{
			HostPath:      RdmaDevices,
			ContainerPath: RdmaDevices,
			Permissions:   "rwm",
		}
		ress[i] = &pluginapi.ContainerAllocateResponse{
			Devices: ds,
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
	if err := os.Remove(m.socket); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// Serve starts the gRPC server and register the device plugin to Kubelet
func (m *RdmaDevPlugin) Serve() error {
	err := m.Start()
	if err != nil {
		log.Printf("Could not start device plugin: %s", err)
		return err
	}
	log.Println("Starting to serve on", m.socket)

	err = m.Register(pluginapi.KubeletSocket, m.resourceName)
	if err != nil {
		log.Printf("Could not register device plugin: %s", err)
		m.Stop()
		return err
	}
	log.Println("Registered device plugin with Kubelet")

	return nil
}
