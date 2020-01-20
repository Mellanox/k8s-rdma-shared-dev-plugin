package resources

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
)

const (
	configFilePath        = "/k8s-rdma-shared-dev-plugin/config.json"
	kubeEndPoint          = "kubelet.sock"
	socketSuffix          = "sock"
	rdmaHcaResourcePrefix = "rdma"
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
	if err = json.Unmarshal(raw, config); err != nil {
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

		if len(conf.Devices) == 0 {
			return fmt.Errorf("error: no devices provided")
		}

		resourceName[conf.ResourceName] = conf.ResourceName
	}

	return nil
}

// InitServers init server
func (rm *resourceManager) InitServers() error {
	for _, config := range rm.configList {
		log.Printf("Resource: %v\n", config)
		rs, err := newResourceServer(config, rm.watchMode, rm.resourcePrefix, rm.socketSuffix)
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
