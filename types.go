package main

import (
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	RdmaSharedDpSuffix = "sock"
	SockDir            = "/var/lib/kubelet/plugins_registry"
	DeprecatedSockDir  = "/var/lib/kubelet/device-plugins"
	KubeEndPoint       = "kubelet.sock"

	RdmaHcaResourcePrefix = "rdma"
)

type UserConfig struct {
	ResourceName string   `json:"resourceName"`
	RdmaHcaMax   int      `json:"rdmaHcaMax"`
	Devices      []string `json:"devices"`
}

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
