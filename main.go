package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"syscall"
)

const (
	ConfigFilePath = "/k8s-rdma-shared-dev-plugin/config.json"
)

const (
	RdmaSharedDpVersion = "0.2"
)

func main() {
	log.Println("Starting K8s RDMA Shared Device Plugin version=", RdmaSharedDpVersion)

	log.Println("Reading", ConfigFilePath)
	raw, err := ioutil.ReadFile(ConfigFilePath)
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	var config UserConfig
	if err = json.Unmarshal(raw, &config); err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}

	s, _ := json.Marshal(config)
	log.Println("loaded config: ", string(s))

	devPlugin, err := NewRdmaSharedDevPlugin(config)
	if err != nil {
		log.Println(err.Error())
		os.Exit(1)
	}
	if err := devPlugin.Start(); err != nil {
		log.Println("Error: Could not contact Kubelet,", err.Error())
		os.Exit(1)
	}

	if !devPlugin.watchMode {
		go devPlugin.Watch()
	}

	log.Println("Starting OS watcher.")
	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case s := <-sigs:
		switch s {
		case syscall.SIGHUP:
			log.Println("Received SIGHUP, restarting.")
			if err = devPlugin.Restart(); err != nil {
				log.Fatalf("unable to restart server %v", err)
			}
		default:
			log.Printf("Received signal \"%v\", shutting down.", s)
			devPlugin.Stop()
			return
		}
	}
}
