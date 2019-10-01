package main

import (
	"log"
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

	rm := newResourceManager()

	log.Println("resource manager reading configs")
	if err := rm.ReadConfig(); err != nil {
		log.Fatalln(err.Error())
	}

	if len(rm.configList) < 1 {
		log.Fatalln("no resource configuration; exiting")
	}

	if !rm.validConfigs() {
		log.Fatalln("Exiting.. one or more invalid configuration(s) given")
	}

	log.Println("Initializing resource servers")
	if err := rm.InitServers(); err != nil {
		log.Fatalf("Error: initializing resource servers %v \n", err)
	}

	log.Println("Starting all servers...")
	if err := rm.StartAllServers(); err != nil {
		log.Fatalf("Error: starting resource servers %v\n", err.Error())
	}

	log.Println("All servers started.")

	log.Println("Listening for term signals")
	log.Println("Starting OS watcher.")
	sigs := newOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	select {
	case s := <-sigs:
		switch s {
		case syscall.SIGHUP:
			log.Println("Received SIGHUP, restarting.")
			if err := rm.RestartAllServers(); err != nil {
				log.Fatalf("unable to restart server %v", err)
			}
		default:
			log.Printf("Received signal \"%v\", shutting down.", s)
			rm.StopAllServers()
			return
		}
	}
}
