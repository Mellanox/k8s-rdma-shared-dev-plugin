package resources

import (
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/utils"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type rdmaDeviceSpec struct {
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
