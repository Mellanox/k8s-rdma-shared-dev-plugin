// Copyright 2025 NVIDIA CORPORATION & AFFILIATES
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

/*----------------------------------------------------

  2023 NVIDIA CORPORATION & AFFILIATES

  Licensed under the Apache License, Version 2.0 (the License);
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an AS IS BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.

----------------------------------------------------*/

package types

import (
	"net"
	"os"

	"github.com/vishvananda/netlink"
	"google.golang.org/grpc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// Selectors contains common device selectors fields
type Selectors struct {
	Vendors   []string `json:"vendors,omitempty"`
	DeviceIDs []string `json:"deviceIDs,omitempty"`
	Drivers   []string `json:"drivers,omitempty"`
	IfNames   []string `json:"ifNames,omitempty"`
	LinkTypes []string `json:"linkTypes,omitempty"`
}

// UserConfig configuration for device plugin
type UserConfig struct {
	ResourceName   string    `json:"resourceName"`
	ResourcePrefix string    `json:"resourcePrefix"`
	RdmaHcaMax     int       `json:"rdmaHcaMax"`
	Devices        []string  `json:"devices"`
	Selectors      Selectors `json:"selectors"`
}

// UserConfigList config list for servers
type UserConfigList struct {
	PeriodicUpdateInterval *int         `json:"periodicUpdateInterval"`
	ConfigList             []UserConfig `json:"configList"`
}

// ResourceServer is gRPC server implements K8s device plugin api
type ResourceServer interface {
	pluginapi.DevicePluginServer
	Start() error
	Stop() error
	Restart() error
	Watch()
	UpdateDevices([]PciNetDevice)
}

// ResourceManager manger multi plugins
type ResourceManager interface {
	ReadConfig() error
	ValidateConfigs() error
	ValidateRdmaSystemMode() error
	DiscoverHostDevices() error
	GetDevices() []PciNetDevice
	InitServers() error
	StartAllServers() error
	StopAllServers() error
	RestartAllServers() error
	GetFilteredDevices(devices []PciNetDevice, selector *Selectors) []PciNetDevice
	PeriodicUpdate() func()
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
	GetClientConn(string) (*grpc.ClientConn, error)
}

// NotifierFactory register signals to listen for
type SignalNotifier interface {
	Notify() chan os.Signal
}

// RdmaDeviceSpec used to find the rdma devices
type RdmaDeviceSpec interface {
	Get(string) []*pluginapi.DeviceSpec
	VerifyRdmaSpec([]*pluginapi.DeviceSpec) error
}

// PciNetDevice provides an interface to get generic device specific information
type PciNetDevice interface {
	GetPciAddr() string
	GetIfName() string
	GetVendor() string
	GetDeviceID() string
	GetDriver() string
	GetLinkType() string
	GetRdmaSpec() []*pluginapi.DeviceSpec
}

// DeviceSelector provides an interface for filtering a list of devices
type DeviceSelector interface {
	Filter([]PciNetDevice) []PciNetDevice
}

// NetlinkManager is an interface to mock nelink library
type NetlinkManager interface {
	LinkByName(string) (netlink.Link, error)
	LinkSetUp(netlink.Link) error
}
