package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/Mellanox/rdmamap"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
)

var (
	sysNetDevices = "/sys/class/net"
	sysBusPci     = "/sys/bus/pci/devices"
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

// GetNetNames returns host net interface names as string for a PCI device from its pci address
func GetNetNames(pciAddr string) ([]string, error) {
	netDir := filepath.Join(sysBusPci, pciAddr, "net")
	if _, err := os.Lstat(netDir); err != nil {
		return nil, fmt.Errorf("no net directory under pci device %s: %q", pciAddr, err)
	}

	fInfos, err := ioutil.ReadDir(netDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read net directory %s: %q", netDir, err)
	}

	names := make([]string, 0)
	for _, f := range fInfos {
		names = append(names, f.Name())
	}

	return names, nil
}

// contains check if a list contains a specific value
func contains(list []string, value string) bool {
	for _, s := range list {
		if s == value {
			return true
		}
	}
	return false
}

// FilterNetDevs filters the devices the are incoming and exists in the allowed devices
func FilterNetDevs(inDevices []types.PciNetDevice, allowed []string) []types.PciNetDevice {
	filteredList := make([]types.PciNetDevice, 0)
	for _, dev := range inDevices {
		if contains(allowed, dev.GetIfName()) {
			filteredList = append(filteredList, dev)
		}
	}
	return filteredList
}
