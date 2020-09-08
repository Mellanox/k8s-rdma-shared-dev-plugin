package resources

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
	"strconv"

	"github.com/jaypipes/ghw"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/utils"
)

const (
	// General constants
	configFilePath        = "/k8s-rdma-shared-dev-plugin/config.json"
	kubeEndPoint          = "kubelet.sock"
	socketSuffix          = "sock"
	rdmaHcaResourcePrefix = "rdma"

	// PCI related constants
	netClass             = 0x02 // Device class - Network controller
	maxVendorNameLength  = 20
	maxProductNameLength = 40
)

var (
	activeSockDir     = "/var/lib/kubelet/plugins_registry"
	deprecatedSockDir = "/var/lib/kubelet/device-plugins"
)

// resourceManager for plugin
type resourceManager struct {
	configFile      string
	resourcePrefix  string
	socketSuffix    string
	watchMode       bool
	configList      []*types.UserConfig
	resourceServers []types.ResourceServer
	deviceList      []*ghw.PCIDevice
}

func NewResourceManager() types.ResourceManager {
	watcherMode := detectPluginWatchMode(activeSockDir)
	if watcherMode {
		fmt.Println("Using Kubelet Plugin Registry Mode")
	} else {
		fmt.Println("Using Deprecated Devie Plugin Registry Path")
	}
	return &resourceManager{
		configFile:     configFilePath,
		resourcePrefix: rdmaHcaResourcePrefix,
		socketSuffix:   socketSuffix,
		watchMode:      watcherMode,
	}
}

// ReadConfig to read configs
func (rm *resourceManager) ReadConfig() error {
	log.Println("Reading", rm.configFile)
	raw, err := ioutil.ReadFile(rm.configFile)
	if err != nil {
		return err
	}

	config := &types.UserConfigList{}
	if err := json.Unmarshal(raw, config); err != nil {
		return err
	}

	log.Printf("loaded config: %+v \n", config.ConfigList)
	for i := range config.ConfigList {
		rm.configList = append(rm.configList, &config.ConfigList[i])
	}
	return nil
}

// ValidateConfigs validate the configurations
func (rm *resourceManager) ValidateConfigs() error {
	resourceName := make(map[string]string)

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

		if conf.RdmaHcaMax < 0 {
			return fmt.Errorf("error: Invalid value for rdmaHcaMax < 0: %d", conf.RdmaHcaMax)
		}

		isEmptySelector := utils.IsEmptySelector(conf.Selectors)
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

// InitServers init server
func (rm *resourceManager) InitServers() error {
	for _, config := range rm.configList {
		log.Printf("Resource: %+v\n", config)
		devices := rm.GetDevices()
		filteredDevices := rm.GetFilteredDevices(devices, config.Selectors)

		if len(filteredDevices) == 0 {
			log.Printf("Warning: no devices in device pool, creating empty resource server for %s", config.ResourceName)
		}

		rs, err := newResourceServer(config, filteredDevices, rm.watchMode, rm.resourcePrefix, rm.socketSuffix)
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
			go func() {
				if err := rs.Watch(); err != nil {
					log.Fatalf("Failed watching socket %v", err)
				}
			}()
		}
	}
	return nil
}

// StopAllServers stop all servers
func (rm *resourceManager) StopAllServers() error {
	for _, rs := range rm.resourceServers {
		if err := rs.Stop(); err != nil {
			return err
		}
	}
	return nil
}

// RestartAllServers restart all servers
func (rm *resourceManager) RestartAllServers() error {
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
	rds := &rdmaDeviceSpec{}
	for _, device := range rm.deviceList {
		newDevice := NewPciNetDevice(device, rds)
		newPciDevices = append(newPciDevices, newDevice)
	}
	return newPciDevices
}

func (rm *resourceManager) GetFilteredDevices(devices []types.PciNetDevice,
	selector types.Selectors) []types.PciNetDevice {
	filteredDevice := devices
	// filter by IfNames list
	if selector.IfNames != nil && len(selector.IfNames) > 0 {
		filteredDevice = NewIfNameSelector(selector.IfNames).Filter(filteredDevice)
	}

	newDeviceList := make([]types.PciNetDevice, len(filteredDevice))
	copy(newDeviceList, filteredDevice)

	return newDeviceList
}
