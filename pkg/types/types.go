package types

import (
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// Selectors contains common device selectors fields
type Selectors struct {
	Vendors   []string `json:"vendors,omitempty"`
	DeviceIDs []string `json:"deviceIDs,omitempty"`
	Drivers   []string `json:"drivers,omitempty"`
	IfNames   []string `json:"ifNames,omitempty"`
}

// UserConfig configuration for device plugin
type UserConfig struct {
	ResourceName string    `json:"resourceName"`
	RdmaHcaMax   int       `json:"rdmaHcaMax"`
	Devices      []string  `json:"devices"`
	Selectors    Selectors `json:"selectors"`
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
	GetFilteredDevices(devices []PciNetDevice, selector *Selectors) []PciNetDevice
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
	GetVendor() string
	GetDeviceID() string
	GetDriver() string
	GetRdmaSpec() []*pluginapi.DeviceSpec
}

// DeviceSelector provides an interface for filtering a list of devices
type DeviceSelector interface {
	Filter([]PciNetDevice) []PciNetDevice
}
