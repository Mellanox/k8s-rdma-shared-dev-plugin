// Copyright 2025 NVIDIA CORPORATION & AFFILIATES
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

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

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"syscall"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/resources"
)

var (
	version = "master@git"
	commit  = "unknown commit"
	date    = "unknown date"
)

func printVersionString() string {
	return fmt.Sprintf("k8s-rdma-shared-dev-plugin version:%s, commit:%s, date:%s", version, commit, date)
}

func main() {
	// Init command line flags to clear vendor packages' flags, especially in init()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	// add version flag
	versionOpt := false
	var configFilePath string
	flag.BoolVar(&versionOpt, "version", false, "Show application version")
	flag.BoolVar(&versionOpt, "v", false, "Show application version")
	flag.StringVar(
		&configFilePath, "config-file", resources.DefaultConfigFilePath, "path to device plugin config file")
	useCdi := false
	flag.BoolVar(&useCdi, "use-cdi", false,
		"Use Container Device Interface to expose devices in containers")
	flag.Parse()
	if versionOpt {
		fmt.Printf("%s\n", printVersionString())
		return
	}

	if useCdi {
		log.Println("CDI enabled")
	}

	log.Println("Starting K8s RDMA Shared Device Plugin version=", version)

	rm := resources.NewResourceManager(configFilePath, useCdi)

	log.Println("resource manager reading configs")
	if err := rm.ReadConfig(); err != nil {
		log.Fatalln(err.Error())
	}

	if err := rm.ValidateConfigs(); err != nil {
		log.Fatalf("Exiting.. one or more invalid configuration(s) given: %v", err)
	}

	if err := rm.ValidateRdmaSystemMode(); err != nil {
		log.Fatalf("Exiting.. can not change : %v", err)
	}

	log.Println("Discovering host devices")
	if err := rm.DiscoverHostDevices(); err != nil {
		log.Fatalf("Error: error discovering host devices %v \n", err)
	}

	log.Println("Initializing resource servers")
	if err := rm.InitServers(); err != nil {
		log.Fatalf("Error: initializing resource servers %v \n", err)
	}

	log.Println("Starting all servers...")
	if err := rm.StartAllServers(); err != nil {
		log.Fatalf("Error: starting resource servers %v\n", err.Error())
	}
	stopPeriodicUpdate := rm.PeriodicUpdate()

	log.Println("All servers started.")

	log.Println("Listening for term signals")
	log.Println("Starting OS watcher.")
	signalsNotifier := resources.NewSignalNotifier(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	sigs := signalsNotifier.Notify()

	s := <-sigs
	switch s {
	case syscall.SIGHUP:
		log.Println("Received SIGHUP, restarting.")
		if err := rm.RestartAllServers(); err != nil {
			log.Fatalf("unable to restart server %v", err)
		}
	default:
		log.Printf("Received signal \"%v\", shutting down.", s)
		stopPeriodicUpdate()
		_ = rm.StopAllServers()
		return
	}
}
