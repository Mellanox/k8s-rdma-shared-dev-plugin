package types

import (
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// UserConfig configuration for device plugin
type UserConfig struct {
	ResourceName string   `json:"resourceName"`
	RdmaHcaMax   int      `json:"rdmaHcaMax"`
	Devices      []string `json:"devices"`
}

// UserConfigList config list for servers
type UserConfigList struct {
	ConfigList []UserConfig `json:"configList"`
}

// ResourceServer is gRPC server implements K8s device plugin api
type ResourceServer interface {
	pluginapi.DevicePluginServer
	Start() error
	Stop() error
	Restart() error
	Watch() error
}

// ResourceManager manger multi plugins
type ResourceManager interface {
	ReadConfig() error
	ValidateConfigs() error
	DiscoverHostDevices() error
	GetDevices() []PciNetDevice
	InitServers() error
	StartAllServers() error
	StopAllServers() error
	RestartAllServers() error
}

// ResourceServerPort to connect the resources server to k8s
type ResourceServerPort interface {
	GetServer() *grpc.Server
	CreateServer()
	DeleteServer()
	Listen(string, string) (net.Listener, error)
	Serve(net.Listener)
	Stop()
	Close(*grpc.ClientConn)
	Register(pluginapi.RegistrationClient, *pluginapi.RegisterRequest) error
	Dial(string, time.Duration) (*grpc.ClientConn, error)
}

// NotifierFactory register signals to listen for
type SignalNotifier interface {
	Notify() chan os.Signal
}

// RdmaDeviceSpec used to find the rdma devices
type RdmaDeviceSpec interface {
	Get(string) []*pluginapi.DeviceSpec
}

// PciNetDevice provides an interface to get generic device specific information
type PciNetDevice interface {
	GetPciAddr() string
	GetIfName() string
	GetRdmaSpec() []*pluginapi.DeviceSpec
}
