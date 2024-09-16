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
