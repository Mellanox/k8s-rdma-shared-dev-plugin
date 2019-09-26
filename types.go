package main

import (
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	RdmaSharedDpSocket = "rdma-shared-dp.sock"
	SockDir           = "/var/lib/kubelet/plugins_registry"
	DeprecatedSockDir = "/var/lib/kubelet/device-plugins"
	KubeEndPoint = "kubelet.sock"

	RdmaHcaResourceName        = "rdma/hca"
)

type UserConfig struct {
	RdmaHcaMax int `json:"rdmaHcaMax"`
}

// RdmaDevPlugin implements the Kubernetes device plugin API
type RdmaDevPlugin struct {
	resourceName    string
	watchMode       bool
	socketName      string
	socketPath      string
	devs            []*pluginapi.Device
	stop            chan interface{}
	stopWatcher     chan bool
	health          chan *pluginapi.Device
	server          *grpc.Server
}

// ResourceServer is gRPC server implements K8s device plugin api
type ResourceServer interface {
	pluginapi.DevicePluginServer
	Start() error
	Stop() error
	Restart() error
	Watch()
}