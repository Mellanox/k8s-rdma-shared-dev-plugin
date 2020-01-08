package types

import (
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	"net"
	"time"
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
	InitServers() error
	StartAllServers() error
	StopAllServers() error
	RestartAllServers() error
}

// ResourceServerConnector to connect the resources server to k8s
type ResourceServerConnector interface {
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
