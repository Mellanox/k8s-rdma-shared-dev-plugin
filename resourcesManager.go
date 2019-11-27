package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"regexp"
)

const (
	configFilePath     = "/k8s-rdma-shared-dev-plugin/config.json"
	rdmaSharedDpSuffix = "sock"
)

// ResourceManager for plugin
type ResourceManager struct {
	configFile        string
	resourcePrefix    string
	socketSuffix      string
	watchMode         bool
	configList        []*UserConfig
	sharedRdmaPlugins []*RdmaDevPlugin
}

func newResourceManager() *ResourceManager {
	watcherMode := detectPluginWatchMode(sockDir)
	if watcherMode {
		fmt.Println("Using Kubelet Plugin Registry Mode")
	} else {
		fmt.Println("Using Deprecated Devie Plugin Registry Path")
	}
	return &ResourceManager{
		configFile:     configFilePath,
		resourcePrefix: rdmaHcaResourcePrefix,
		socketSuffix:   rdmaSharedDpSuffix,
		watchMode:      watcherMode,
	}
}

// ReadConfig to read configs
func (rm *ResourceManager) ReadConfig() error {
	log.Println("Reading", rm.configFile)
	raw, err := ioutil.ReadFile(rm.configFile)
	if err != nil {
		return err
	}

	config := &UserConfigList{}
	if err = json.Unmarshal(raw, config); err != nil {
		return err
	}

	log.Printf("loaded config: %+v \n", config.ConfigList)
	for i := range config.ConfigList {
		rm.configList = append(rm.configList, &config.ConfigList[i])
	}
	return nil
}

func (rm *ResourceManager) validConfigs() bool {
	resourceName := make(map[string]string) // resource name placeholder

	for _, conf := range rm.configList {
		// check if name contains acceptable characters
		if !validResourceName(conf.ResourceName) {
			log.Printf("Error: resource name \"%s\" contains invalid characters \n", conf.ResourceName)
			return false
		}
		// check resource names are unique
		_, ok := resourceName[conf.ResourceName]
		if ok {
			// resource name already exist
			log.Printf("Error: resource name \"%s\" already exists \n", conf.ResourceName)
			return false
		}

		resourceName[conf.ResourceName] = conf.ResourceName
	}

	return true
}

// InitServers init server
func (rm *ResourceManager) InitServers() error {
	for _, config := range rm.configList {
		log.Printf("Resource: %v\n", config)
		rs, err := NewRdmaSharedDevPlugin(config, rm.watchMode, rm.resourcePrefix, rm.socketSuffix)
		if err != nil {
			return err
		}
		rm.sharedRdmaPlugins = append(rm.sharedRdmaPlugins, rs)
	}
	return nil
}

// StartAllServers start all servers
func (rm *ResourceManager) StartAllServers() error {
	for _, rs := range rm.sharedRdmaPlugins {
		log.Printf("Resource: %v\n", rs.resourceName)
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
func (rm *ResourceManager) StopAllServers() error {
	for _, rs := range rm.sharedRdmaPlugins {
		if err := rs.Stop(); err != nil {
			return err
		}
	}
	return nil
}

// RestartAllServers restart all servers
func (rm *ResourceManager) RestartAllServers() error {
	for _, rs := range rm.sharedRdmaPlugins {
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
