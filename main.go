package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"syscall"

	"github.com/fsnotify/fsnotify"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

const (
	ConfigFilePath = "/k8s-rdma-shared-dev-plugin/config.json"
)

const (
	RdmaSharedDpVersion = "0.2"
)

func main() {
	log.Println("Starting K8s RDMA Shared Device Plugin version=", RdmaSharedDpVersion)

	log.Println("Starting FS watcher.")
	watcher, err := newFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		log.Println("Failed to created FS watcher.")
		os.Exit(1)
	}
	defer watcher.Close()

	log.Println("Starting OS watcher.")
	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	log.Println("Reading", ConfigFilePath)
	raw, err := ioutil.ReadFile(ConfigFilePath)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	var config UserConfig
	json.Unmarshal(raw, &config)

	s, _ := json.Marshal(config)
	log.Println("loaded config: ", string(s))

	restart := true
	var devPlugin *RdmaDevPlugin

L:
	for {
		if restart {
			if devPlugin != nil {
				devPlugin.Stop()
			}
			devPlugin = NewRdmaSharedDevPlugin(config)
			if err := devPlugin.Serve(); err != nil {
				log.Println("Could not contact Kubelet, retrying. Did you enable the device plugin feature gate?")
			} else {
				restart = false
			}
		}

		select {
		case event := <-watcher.Events:
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("inotify: %s created, restarting.", pluginapi.KubeletSocket)
				restart = true
			}

		case err := <-watcher.Errors:
			log.Printf("inotify: %s", err)

		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				log.Println("Received SIGHUP, restarting.")
				restart = true
			default:
				log.Printf("Received signal \"%v\", shutting down.", s)
				devPlugin.Stop()
				break L
			}
		}
	}
}
