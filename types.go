package main

import (
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	sockDir           = "/var/lib/kubelet/plugins_registry"
	deprecatedSockDir = "/var/lib/kubelet/device-plugins"
	kubeEndPoint      = "kubelet.sock"

	rdmaHcaResourcePrefix = "rdma"
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

// RdmaDevPlugin implements the Kubernetes device plugin API
type RdmaDevPlugin struct {
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

// ResourceServer is gRPC server implements K8s device plugin api
type ResourceServer interface {
	pluginapi.DevicePluginServer
	Start() error
	Stop() error
	Restart() error
	Watch()
}
