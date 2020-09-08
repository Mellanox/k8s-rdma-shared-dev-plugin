package resources

import (
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
)

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
