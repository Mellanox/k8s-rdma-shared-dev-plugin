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
	vendor     string
	deviceID   string
	driver     string
	linkType   string
	rdmaSpec   []*pluginapi.DeviceSpec
}

// NewPciNetDevice returns an instance of PciNetDevice interface
func NewPciNetDevice(dev *ghw.PCIDevice, rds types.RdmaDeviceSpec,
	nLink types.NetlinkManager) (types.PciNetDevice, error) {
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

	driver, err := utils.GetPCIDevDriver(pciAddr)
	if err != nil {
		return nil, err
	}

	linkType := ""
	if len(ifName) > 0 {
		link, err := nLink.LinkByName(ifName)
		if err != nil {
			return nil, err
		}
		linkType = link.Attrs().EncapType
	}

	rdmaSpec := rds.Get(pciAddr)

	return &pciNetDevice{
		pciAddress: pciAddr,
		vendor:     dev.Vendor.ID,
		deviceID:   dev.Product.ID,
		driver:     driver,
		ifName:     ifName,
		linkType:   linkType,
		rdmaSpec:   rdmaSpec,
	}, nil
}

func (nd *pciNetDevice) GetVendor() string {
	return nd.vendor
}

func (nd *pciNetDevice) GetDeviceID() string {
	return nd.deviceID
}

func (nd *pciNetDevice) GetDriver() string {
	return nd.driver
}

func (nd *pciNetDevice) GetLinkType() string {
	return nd.linkType
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
