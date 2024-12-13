/*----------------------------------------------------

  2023 NVIDIA CORPORATION & AFFILIATES

  Licensed under the Apache License, Version 2.0 (the License);
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an AS IS BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.

----------------------------------------------------*/

package resources

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/jaypipes/ghw"
	"github.com/vishvananda/netlink"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/cdi"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/utils"
)

const (
	// General constants
	DefaultConfigFilePath = "/k8s-rdma-shared-dev-plugin/config.json"
	kubeEndPoint          = "kubelet.sock"
	socketSuffix          = "sock"
	rdmaHcaResourcePrefix = "rdma"

	// PCI related constants
	netClass             = 0x02 // Device class - Network controller
	maxVendorNameLength  = 20
	maxProductNameLength = 40

	// Default periodic update interval
	defaultPeriodicUpdateInterval = 60 * time.Second

	// RDMA subsystem network namespace mode
	rdmaExclusive = "exclusive"
)

var (
	activeSockDir     = "/var/lib/kubelet/plugins_registry"
	deprecatedSockDir = "/var/lib/kubelet/device-plugins"
)

// resourceManager for plugin
type resourceManager struct {
	configFile             string
	defaultResourcePrefix  string
	socketSuffix           string
	watchMode              bool
	configList             []*types.UserConfig
	resourceServers        []types.ResourceServer
	deviceList             []*ghw.PCIDevice
	netlinkManager         types.NetlinkManager
	rds                    types.RdmaDeviceSpec
	PeriodicUpdateInterval time.Duration
	useCdi                 bool
}

func NewResourceManager(configFile string, useCdi bool) types.ResourceManager {
	watcherMode := detectPluginWatchMode(activeSockDir)
	if watcherMode {
		fmt.Println("Using Kubelet Plugin Registry Mode")
	} else {
		fmt.Println("Using Deprecated Devie Plugin Registry Path")
	}
	return &resourceManager{
		configFile:            configFile,
		defaultResourcePrefix: rdmaHcaResourcePrefix,
		socketSuffix:          socketSuffix,
		watchMode:             watcherMode,
		netlinkManager:        &netlinkManager{},
		rds:                   NewRdmaDeviceSpec(requiredRdmaDevices),
		useCdi:                useCdi,
	}
}

// ReadConfig to read configs
func (rm *resourceManager) ReadConfig() error {
	log.Println("Reading", rm.configFile)
	raw, err := os.ReadFile(rm.configFile)
	if err != nil {
		return err
	}

	config := &types.UserConfigList{}
	if err := json.Unmarshal(raw, config); err != nil {
		return err
	}

	log.Printf("loaded config: %+v \n", config.ConfigList)

	// if periodic update is not set then use the default value
	if config.PeriodicUpdateInterval == nil {
		log.Println("no periodic update interval is set, use default interval 60 seconds")
		rm.PeriodicUpdateInterval = defaultPeriodicUpdateInterval
	} else {
		PeriodicUpdateInterval := *config.PeriodicUpdateInterval
		if PeriodicUpdateInterval == 0 {
			log.Println("warning: periodic update interval is 0, no periodic update will run")
		} else {
			log.Printf("periodic update interval: %+d \n", PeriodicUpdateInterval)
		}
		rm.PeriodicUpdateInterval = time.Duration(PeriodicUpdateInterval) * time.Second
	}

	for i := range config.ConfigList {
		rm.configList = append(rm.configList, &config.ConfigList[i])
	}
	return nil
}

// ValidateConfigs validate the configurations
func (rm *resourceManager) ValidateConfigs() error {
	resourceName := make(map[string]string)

	if rm.PeriodicUpdateInterval < 0 {
		return fmt.Errorf("invalid \"periodicUpdateInterval\" configuration \"%d\"", rm.PeriodicUpdateInterval)
	}

	if len(rm.configList) < 1 {
		return fmt.Errorf("no resources configuration found")
	}

	for _, conf := range rm.configList {
		// check if name contains acceptable characters
		if !validResourceName(conf.ResourceName) {
			return fmt.Errorf("error: resource name \"%s\" contains invalid characters", conf.ResourceName)
		}
		// check resource names are unique
		_, ok := resourceName[conf.ResourceName]
		if ok {
			// resource name already exist
			return fmt.Errorf("error: resource name \"%s\" already exists", conf.ResourceName)
		}
		// If prefix is not configured - use the default one
		if conf.ResourcePrefix == "" {
			conf.ResourcePrefix = rm.defaultResourcePrefix
		}

		if conf.RdmaHcaMax < 0 {
			return fmt.Errorf("error: Invalid value for rdmaHcaMax < 0: %d", conf.RdmaHcaMax)
		}

		isEmptySelector := utils.IsEmptySelector(&(conf.Selectors))
		if isEmptySelector && len(conf.Devices) == 0 {
			return fmt.Errorf("error: configuration missmatch. neither \"selectors\" nor \"devices\" fields exits," +
				" it is recommended to use the new “selectors” field")
		}

		// If both "selectors" and "devices" fields are provided then fail with confusion
		if !isEmptySelector && len(conf.Devices) > 0 {
			return fmt.Errorf("configuration mismatch. Cannot specify both \"selectors\" and \"devices\" fields")
		} else if isEmptySelector { // If no "selector" then use devices as IfNames selector
			log.Println("Warning: \"devices\" field is deprecated, it is recommended to use the new “selectors” field")
			conf.Selectors.IfNames = conf.Devices
		}

		resourceName[conf.ResourceName] = conf.ResourceName
	}

	return nil
}

// ValidateRdmaSystemMode ensure RDMA subsystem network namespace mode is shared
func (rm *resourceManager) ValidateRdmaSystemMode() error {
	mode, err := netlink.RdmaSystemGetNetnsMode()
	if err != nil {
		if err.Error() == "invalid argument" {
			log.Printf("too old kernel to get RDMA subsystem")
			return nil
		}

		return fmt.Errorf("can not get RDMA subsystem network namespace mode")
	}

	if mode == rdmaExclusive {
		return fmt.Errorf("incorrect RDMA subsystem network namespace")
	}
	return nil
}

// InitServers init server
func (rm *resourceManager) InitServers() error {
	for _, config := range rm.configList {
		log.Printf("Resource: %+v\n", config)
		devices := rm.GetDevices()
		filteredDevices := rm.GetFilteredDevices(devices, &config.Selectors)

		if len(filteredDevices) == 0 {
			log.Printf("Warning: no devices in device pool, creating empty resource server for %s", config.ResourceName)
		}

		if rm.useCdi {
			err := cdi.CleanupSpecs(cdiResourcePrefix)
			if err != nil {
				return err
			}
		}
		rs, err := newResourceServer(config, filteredDevices, rm.watchMode, rm.socketSuffix, rm.useCdi)
		if err != nil {
			return err
		}
		rm.resourceServers = append(rm.resourceServers, rs)
	}
	return nil
}

// StartAllServers start all servers
func (rm *resourceManager) StartAllServers() error {
	for _, rs := range rm.resourceServers {
		if err := rs.Start(); err != nil {
			return err
		}

		// start watcher
		if !rm.watchMode {
			go rs.Watch()
		}
	}
	return nil
}

// StopAllServers stop all servers
func (rm *resourceManager) StopAllServers() error {
	if rm.useCdi {
		err := cdi.CleanupSpecs(cdiResourcePrefix)
		if err != nil {
			return err
		}
	}
	for _, rs := range rm.resourceServers {
		if err := rs.Stop(); err != nil {
			return err
		}
	}
	return nil
}

// RestartAllServers restart all servers
func (rm *resourceManager) RestartAllServers() error {
	if rm.useCdi {
		err := cdi.CleanupSpecs(cdiResourcePrefix)
		if err != nil {
			return err
		}
	}
	for _, rs := range rm.resourceServers {
		if err := rs.Restart(); err != nil {
			return err
		}
	}
	return nil
}

func validResourceName(name string) bool {
	// name regex
	var validString = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	return validString.MatchString(name)
}

func (rm *resourceManager) DiscoverHostDevices() error {
	log.Println("discovering host network devices")
	pci, err := ghw.PCI()
	if err != nil {
		return fmt.Errorf("error getting PCI info: %v", err)
	}

	devices := pci.ListDevices()
	if len(devices) == 0 {
		log.Println("Warning: DiscoverHostDevices(): no PCI network device found")
	}

	// cleanup deviceList as this method is also called during periodic update for the resources
	rm.deviceList = []*ghw.PCIDevice{}

	for _, device := range devices {
		devClass, err := strconv.ParseInt(device.Class.ID, 16, 64)
		if err != nil {
			log.Printf("Warning: DiscoverHostDevices(): unable to parse device class for device %+v %q", device,
				err)
			continue
		}

		if devClass != netClass {
			continue
		}

		vendor := device.Vendor
		vendorName := vendor.Name
		if len(vendor.Name) > maxVendorNameLength {
			vendorName = string([]byte(vendorName)[0:17]) + "..."
		}
		product := device.Product
		productName := product.Name
		if len(product.Name) > maxProductNameLength {
			productName = string([]byte(productName)[0:37]) + "..."
		}
		log.Printf("DiscoverHostDevices(): device found: %-12s\t%-12s\t%-20s\t%-40s", device.Address,
			device.Class.ID, vendorName, productName)

		rm.deviceList = append(rm.deviceList, device)
	}

	return nil
}

func (rm *resourceManager) GetDevices() []types.PciNetDevice {
	newPciDevices := make([]types.PciNetDevice, 0)
	for _, device := range rm.deviceList {
		if newDevice, err := NewPciNetDevice(device, rm.rds, rm.netlinkManager); err == nil {
			newPciDevices = append(newPciDevices, newDevice)
		} else {
			log.Printf("error creating new device: %q", err)
		}
	}
	return newPciDevices
}

func (rm *resourceManager) GetFilteredDevices(devices []types.PciNetDevice,
	selector *types.Selectors) []types.PciNetDevice {
	filteredDevice := devices

	// filter by Vendors list
	if selector.Vendors != nil && len(selector.Vendors) > 0 {
		filteredDevice = NewVendorSelector(selector.Vendors).Filter(filteredDevice)
	}

	// filter by DeviceIDs list
	if selector.DeviceIDs != nil && len(selector.DeviceIDs) > 0 {
		filteredDevice = NewDeviceSelector(selector.DeviceIDs).Filter(filteredDevice)
	}

	// filter by Driver list
	if selector.Drivers != nil && len(selector.Drivers) > 0 {
		filteredDevice = NewDriverSelector(selector.Drivers).Filter(filteredDevice)
	}

	// filter by IfNames list
	if selector.IfNames != nil && len(selector.IfNames) > 0 {
		filteredDevice = NewIfNameSelector(selector.IfNames).Filter(filteredDevice)
	}

	// filter by LinkType list
	if selector.LinkTypes != nil && len(selector.LinkTypes) > 0 {
		filteredDevice = NewLinkTypeSelector(selector.LinkTypes).Filter(filteredDevice)
	}

	newDeviceList := make([]types.PciNetDevice, len(filteredDevice))
	copy(newDeviceList, filteredDevice)

	return newDeviceList
}

func (rm *resourceManager) PeriodicUpdate() func() {
	stopChan := make(chan interface{})
	if rm.PeriodicUpdateInterval > 0 {
		ticker := time.NewTicker(rm.PeriodicUpdateInterval)

		// Listen for update or stop update channels
		go func() {
			for {
				select {
				case <-ticker.C:
					if err := rm.DiscoverHostDevices(); err != nil {
						log.Printf("error: failed to discover host devices: %v", err)
						continue
					}

					for index, rs := range rm.resourceServers {
						devices := rm.GetDevices()
						filteredDevices := rm.GetFilteredDevices(devices, &rm.configList[index].Selectors)
						rs.UpdateDevices(filteredDevices)
					}
				case <-stopChan:
					ticker.Stop()
					return
				}
			}
		}()
	}
	// Return stop function
	return func() {
		if rm.PeriodicUpdateInterval > 0 {
			stopChan <- true
			close(stopChan)
		}
	}
}
