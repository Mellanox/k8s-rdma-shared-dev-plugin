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

package utils

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"

	"github.com/Mellanox/rdmamap"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
)

var (
	sysNetDevices = "/sys/class/net"
	SysBusPci     = "/sys/bus/pci/devices"
)

// GetPciAddress return the pci address for given interface name
func GetPciAddress(ifName string) (string, error) {
	var pciAddress string
	ifaceDir := path.Join(sysNetDevices, ifName, "device")
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

// GetRdmaDevices return rdma devices for given device pci address
func GetRdmaDevices(pciAddress string) []string {
	rdmaResources := rdmamap.GetRdmaDevicesForPcidev(pciAddress)
	rdmaDevices := make([]string, 0, len(rdmaResources))
	for _, resource := range rdmaResources {
		rdmaResourceDevices := rdmamap.GetRdmaCharDevices(resource)
		rdmaDevices = append(rdmaDevices, rdmaResourceDevices...)
	}

	return rdmaDevices
}

// IsEmptySelector returns if the selector is empty
func IsEmptySelector(selector *types.Selectors) bool {
	values := reflect.ValueOf(*selector)

	for i := 0; i < values.NumField(); i++ {
		value := values.Field(i)
		if !value.IsNil() && value.Len() > 0 {
			return false
		}
	}
	return true
}

// GetNetNames returns host net interface names as string for a PCI device from its pci address
func GetNetNames(pciAddr string) ([]string, error) {
	netDir := filepath.Join(SysBusPci, pciAddr, "net")
	if _, err := os.Lstat(netDir); err != nil {
		return nil, fmt.Errorf("no net directory under pci device %s: %q", pciAddr, err)
	}

	fInfos, err := os.ReadDir(netDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read net directory %s: %q", netDir, err)
	}

	names := make([]string, 0)
	for _, f := range fInfos {
		names = append(names, f.Name())
	}

	return names, nil
}

// GetPCIDevDriver returns current driver attached to a pci device from its pci address
func GetPCIDevDriver(pciAddr string) (string, error) {
	driverLink := filepath.Join(SysBusPci, pciAddr, "driver")
	driverInfo, err := os.Readlink(driverLink)
	if err != nil {
		return "", fmt.Errorf("error getting driver info for device %s %v", pciAddr, err)
	}
	return filepath.Base(driverInfo), nil
}
