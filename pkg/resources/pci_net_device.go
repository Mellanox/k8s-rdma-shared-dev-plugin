package resources

import (
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
func NewPciNetDevice(ifName string, rds types.RdmaDeviceSpec) (types.PciNetDevice, error) {
	pciAddr, err := utils.GetPciAddress(ifName)
	if err != nil {
		return nil, err
	}

	rdmaSpec := rds.Get(pciAddr)

	return &pciNetDevice{
		pciAddress: pciAddr,
		ifName:     ifName,
		rdmaSpec:   rdmaSpec,
	}, nil
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
