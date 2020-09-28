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

type driverSelector struct {
	drivers []string
}

// NewDriverSelector returns a DeviceSelector interface for driver list
func NewDriverSelector(drivers []string) types.DeviceSelector {
	return &driverSelector{drivers: drivers}
}

func (s *driverSelector) Filter(inDevices []types.PciNetDevice) []types.PciNetDevice {
	filteredList := make([]types.PciNetDevice, 0)
	for _, dev := range inDevices {
		devDriver := dev.GetDriver()
		if contains(s.drivers, devDriver) {
			filteredList = append(filteredList, dev)
		}
	}
	return filteredList
}

type linkTypeSelector struct {
	linkTypes []string
}

// NewLinkTypeSelector returns a interface for netDev list
func NewLinkTypeSelector(linkTypes []string) types.DeviceSelector {
	return &linkTypeSelector{linkTypes: linkTypes}
}

func (s *linkTypeSelector) Filter(inDevices []types.PciNetDevice) []types.PciNetDevice {
	filteredList := make([]types.PciNetDevice, 0)
	for _, dev := range inDevices {
		linkType := dev.GetLinkType()
		if contains(s.linkTypes, linkType) {
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
