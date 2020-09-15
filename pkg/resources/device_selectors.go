package resources

import (
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
)

type vendorSelector struct {
	vendors []string
}

// NewVendorSelector returns a DeviceSelector interface for vendor list
func NewVendorSelector(vendors []string) types.DeviceSelector {
	return &vendorSelector{vendors: vendors}
}

func (s *vendorSelector) Filter(inDevices []types.PciNetDevice) []types.PciNetDevice {
	filteredList := make([]types.PciNetDevice, 0)
	for _, dev := range inDevices {
		devVendor := dev.GetVendor()
		if contains(s.vendors, devVendor) {
			filteredList = append(filteredList, dev)
		}
	}
	return filteredList
}

type deviceIDSelector struct {
	devices []string
}

// NewDeviceSelector returns a DeviceSelector interface for device id list
func NewDeviceSelector(devices []string) types.DeviceSelector {
	return &deviceIDSelector{devices: devices}
}

func (s *deviceIDSelector) Filter(inDevices []types.PciNetDevice) []types.PciNetDevice {
	filteredList := make([]types.PciNetDevice, 0)
	for _, dev := range inDevices {
		devCode := dev.GetDeviceID()
		if contains(s.devices, devCode) {
			filteredList = append(filteredList, dev)
		}
	}
	return filteredList
}

type ifNameSelector struct {
	ifNames []string
}

// NewIfNameSelector returns a DeviceSelector interface for ifName list
func NewIfNameSelector(ifNames []string) types.DeviceSelector {
	return &ifNameSelector{ifNames: ifNames}
}

func (s *ifNameSelector) Filter(inDevices []types.PciNetDevice) []types.PciNetDevice {
	filteredList := make([]types.PciNetDevice, 0)
	for _, dev := range inDevices {
		if contains(s.ifNames, dev.GetIfName()) {
			filteredList = append(filteredList, dev)
		}
	}
	return filteredList
}

func contains(list []string, needle string) bool {
	for _, s := range list {
		if s == needle {
			return true
		}
	}
	return false
}
