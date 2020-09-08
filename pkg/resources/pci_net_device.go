package resources

import (
	"log"

	"github.com/jaypipes/ghw"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/utils"
)

// pciNetDevice implements PciNetDevice interface to get generic device specific information
type pciNetDevice struct {
	pciAddress string
	ifName     string
	rdmaSpec   []*pluginapi.DeviceSpec
}

// NewPciNetDevice returns an instance of PciNetDevice interface
func NewPciNetDevice(dev *ghw.PCIDevice, rds types.RdmaDeviceSpec) types.PciNetDevice {
	var ifName string

	pciAddr := dev.Address
	netDevs, _ := utils.GetNetNames(pciAddr)
	if len(netDevs) == 0 {
		ifName = ""
	} else {
		ifName = netDevs[0]
		if len(netDevs) > 1 {
			log.Printf("Warning: found several names for device %s %v, using first name %s", pciAddr, netDevs,
				ifName)
		}
	}

	rdmaSpec := rds.Get(pciAddr)

	return &pciNetDevice{
		pciAddress: pciAddr,
		ifName:     ifName,
		rdmaSpec:   rdmaSpec,
	}
}

func (nd *pciNetDevice) GetIfName() string {
	return nd.ifName
}

func (nd *pciNetDevice) GetPciAddr() string {
	return nd.pciAddress
}

func (nd *pciNetDevice) GetRdmaSpec() []*pluginapi.DeviceSpec {
	return nd.rdmaSpec
}
