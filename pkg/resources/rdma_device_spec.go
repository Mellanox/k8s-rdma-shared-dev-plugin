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

package resources

import (
	"fmt"
	"path"
	"strings"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/utils"
)

type rdmaDeviceSpec struct {
	rdmaDevs []string
}

// Required RDMA devices
var requiredRdmaDevices = []string{"rdma_cm", "umad", "uverbs"}

func NewRdmaDeviceSpec(rdmaDevs []string) types.RdmaDeviceSpec {
	return &rdmaDeviceSpec{rdmaDevs: rdmaDevs}
}

func (rf *rdmaDeviceSpec) Get(pciAddress string) []*pluginapi.DeviceSpec {
	deviceSpec := make([]*pluginapi.DeviceSpec, 0)

	rdmaDevices := utils.GetRdmaDevices(pciAddress)
	for _, device := range rdmaDevices {
		deviceSpec = append(deviceSpec, &pluginapi.DeviceSpec{
			HostPath:      device,
			ContainerPath: device,
			Permissions:   "rwm"})
	}

	return deviceSpec
}

func (rf *rdmaDeviceSpec) VerifyRdmaSpec(rdmaDevSpecs []*pluginapi.DeviceSpec) error {
	for _, rdmaDev := range rf.rdmaDevs {
		if !containsRdmaDev(rdmaDevSpecs, rdmaDev) {
			return fmt.Errorf("RDMA device %q not found", rdmaDev)
		}
	}

	return nil
}

func containsRdmaDev(devSpecs []*pluginapi.DeviceSpec, rdmaDev string) bool {
	for _, devSpec := range devSpecs {
		_, devSpecName := path.Split(devSpec.HostPath)
		if strings.Contains(devSpecName, rdmaDev) {
			return true
		}
	}

	return false
}
