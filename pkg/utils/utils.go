package utils

import (
	"fmt"
	"github.com/Mellanox/rdmamap"
	"os"
	"path"
)

var (
	sysNetDevices = "/sys/class/net"
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

//GetRdmaDevices return rdma devices for given device pci address
func GetRdmaDevices(pciAddress string) []string {
	var rdmaDevices []string

	rdmaResources := rdmamap.GetRdmaDevicesForPcidev(pciAddress)
	for _, resource := range rdmaResources {
		rdmaResourceDevices := rdmamap.GetRdmaCharDevices(resource)
		rdmaDevices = append(rdmaDevices, rdmaResourceDevices...)
	}

	return rdmaDevices
}
