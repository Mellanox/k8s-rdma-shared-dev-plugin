// Copyright 2026 NVIDIA CORPORATION & AFFILIATES
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

package resources

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	cdiLib "github.com/container-orchestrated-devices/container-device-interface/pkg/cdi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	registerapi "k8s.io/kubelet/pkg/apis/pluginregistration/v1"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types/mocks"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/cdi"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
)

func TestResourceNamesWithinPrefixes(t *testing.T) {
	for _, test := range []struct {
		name     string
		prefixes []string
		valid    bool
	}{
		{"different prefixes", []string{"a.example", "b.example"}, true},
		{"same prefix", []string{"a.example", "a.example"}, false},
		{"implicit default", []string{"", "rdma"}, false},
		{"two implicit defaults", []string{"", ""}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			rm := &resourceManager{defaultResourcePrefix: "rdma"}
			for index, prefix := range test.prefixes {
				rm.configList = append(rm.configList, &types.UserConfig{
					ResourceName: "hca", ResourcePrefix: prefix, RdmaHcaMax: 2,
					Selectors: types.Selectors{IfNames: []string{fmt.Sprintf("ib%d", index)}},
				})
			}
			err := rm.ValidateConfigs()
			if (err == nil) != test.valid {
				t.Fatalf("ValidateConfigs() = %v, want valid=%t", err, test.valid)
			}
		})
	}
}

func prefixConfig(prefix, name, ifName string) *types.UserConfig {
	return &types.UserConfig{ResourcePrefix: prefix, ResourceName: name, RdmaHcaMax: 2,
		Selectors: types.Selectors{IfNames: []string{ifName}}}
}

func prefixManager(t *testing.T, root string, configs []*types.UserConfig, useCDI bool) *resourceManager {
	t.Helper()
	if root == "" {
		var err error
		root, err = os.MkdirTemp("", "rdma-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(root) })
	}
	if err := os.MkdirAll(getActiveSockDir(root), 0700); err != nil {
		t.Fatal(err)
	}
	rm := &resourceManager{defaultResourcePrefix: "rdma", kubeletRootDir: root,
		watchMode: true, useCdi: useCDI, configList: configs}
	if err := rm.ValidateConfigs(); err != nil {
		t.Fatal(err)
	}
	if err := rm.InitServers(); err != nil {
		t.Fatal(err)
	}
	return rm
}

func prefixCDIRegistry(t *testing.T) cdiLib.Registry {
	t.Helper()
	registry := cdiLib.GetRegistry()
	oldDirs := append([]string(nil), registry.GetSpecDirectories()...)
	if err := registry.Configure(cdiLib.WithSpecDirs(t.TempDir(), t.TempDir())); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := registry.Configure(cdiLib.WithSpecDirs(oldDirs...)); err != nil {
			t.Error(err)
		}
	})
	return registry
}

func prefixDevices(t *testing.T) []types.PciNetDevice {
	t.Helper()
	devices := make([]types.PciNetDevice, 2)
	for index := range devices {
		device := mocks.NewMockPciNetDevice(t)
		device.On("GetIfName").Return(fmt.Sprintf("ib%d", index)).Maybe()
		device.On("GetPciAddr").Return(fmt.Sprintf("0000:00:00.%d", index+1)).Maybe()
		node := fmt.Sprintf("/dev/infiniband/uverbs%d", index)
		device.On("GetRdmaSpec").Return([]*pluginapi.DeviceSpec{{HostPath: node, ContainerPath: node}}).Maybe()
		devices[index] = device
	}
	return devices
}

func prefixConnection(t *testing.T, rs *resourceServer) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient("unix://"+rs.socketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func prefixAllocation(t *testing.T, conn *grpc.ClientConn, resourceName string) map[string]string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	info, err := registerapi.NewRegistrationClient(conn).GetInfo(ctx, &registerapi.InfoRequest{})
	if err != nil || info.GetName() != resourceName {
		t.Fatalf("GetInfo() = %v, %v; want %s", info, err, resourceName)
	}
	client := pluginapi.NewDevicePluginClient(conn)
	stream, err := client.ListAndWatch(ctx, &pluginapi.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	devices, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	if len(devices.Devices) != 2 || devices.Devices[0].ID != "0" || devices.Devices[1].ID != "1" {
		t.Fatalf("virtual device IDs changed: %v", devices)
	}
	allocation, err := client.Allocate(ctx, &pluginapi.AllocateRequest{
		ContainerRequests: []*pluginapi.ContainerAllocateRequest{{DevicesIds: []string{"0"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return allocation.ContainerResponses[0].Annotations
}

func assertPrefixReferences(t *testing.T, registry cdiLib.Registry, annotations map[string]string, poolName string) {
	t.Helper()
	if err := registry.Refresh(); err != nil {
		t.Fatal(err)
	}
	_, devices, err := cdiLib.ParseAnnotations(annotations)
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 {
		t.Fatalf("merged annotations lost a pool: %v", devices)
	}
	for index, prefix := range []string{"a.example", "b.example"} {
		poolID := resourcePoolID(prefix + "/" + poolName)
		name := fmt.Sprintf("nvidia.com/net-rdma=%s_0000:00:00.%d", poolID, index+1)
		device := registry.DeviceDB().GetDevice(name)
		if device == nil {
			t.Fatalf("qualified CDI name %s no longer resolves", name)
		}
		nodes := device.ContainerEdits.DeviceNodes
		expected := fmt.Sprintf("/dev/infiniband/uverbs%d", index)
		if len(nodes) != 1 || nodes[0].HostPath != expected {
			t.Fatalf("CDI reference %s resolved to the wrong pool: %v", name, nodes)
		}
		found := false
		for _, reference := range devices {
			found = found || reference == name
		}
		if !found {
			t.Fatalf("Allocate did not return qualified CDI name %s: %v", name, devices)
		}
	}
}

func prefixInitializeDevices(t *testing.T, rm *resourceManager, devices []types.PciNetDevice) {
	t.Helper()
	for index, server := range rm.resourceServers {
		config := rm.configList[index]
		filtered := rm.GetFilteredDevices(devices, &config.Selectors)
		initialized, err := newResourceServer(config, filtered, rm.kubeletRootDir, true, true,
			server.(*resourceServer).socketName)
		if err != nil {
			t.Fatal(err)
		}
		rm.resourceServers[index] = initialized
	}
}

func TestPrefixPoolsEndpointsAndCDI(t *testing.T) {
	for _, name := range []string{"hca", strings.Repeat("h", 63)} {
		t.Run(name, func(t *testing.T) { testPrefixPoolsEndpointsAndCDI(t, name) })
	}
}

func testPrefixPoolsEndpointsAndCDI(t *testing.T, name string) {
	registry := prefixCDIRegistry(t)
	devices := prefixDevices(t)
	if err := cdi.New().CreateCDISpec(cdiResourcePrefix, cdiResourceKind, name, devices[:1], devices[:1]); err != nil {
		t.Fatal(err)
	}
	oldSpec := filepath.Join(registry.GetSpecDirectories()[1], "nvidia.com_"+name+".yaml")
	rm := prefixManager(t, "", []*types.UserConfig{
		prefixConfig("a.example", name, "ib0"), prefixConfig("b.example", name, "ib1"),
	}, true)
	legacyPath := filepath.Join(getActiveSockDir(rm.kubeletRootDir), name+".sock")
	legacy, listenErr := net.Listen("unix", legacyPath)
	if listenErr != nil {
		t.Fatal(listenErr)
	}
	t.Cleanup(func() { _ = legacy.Close() })
	if _, err := os.Stat(oldSpec); !os.IsNotExist(err) {
		t.Fatalf("legacy spec survived startup and could conflict with its replacement: %v", err)
	}
	prefixInitializeDevices(t, rm, devices)
	if err := rm.StartAllServers(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := rm.StopAllServers(); err != nil {
			t.Error(err)
		}
	})
	first := rm.resourceServers[0].(*resourceServer)
	second := rm.resourceServers[1].(*resourceServer)
	if first.socketPath == second.socketPath || first.cdiResourceName == second.cdiResourceName {
		t.Fatal("different prefixed pools share an internal identity")
	}
	conns := []*grpc.ClientConn{prefixConnection(t, first), prefixConnection(t, second)}
	merged := make(map[string]string)
	for index, server := range []*resourceServer{first, second} {
		if err := server.updateCDISpec(); err != nil {
			t.Fatal(err)
		}
		for key, value := range prefixAllocation(t, conns[index], server.resourceName) {
			merged[key] = value
		}
	}
	assertPrefixReferences(t, registry, merged, name)
	otherPath := filepath.Join(registry.GetSpecDirectories()[1], "nvidia.com_"+second.cdiResourceName+".yaml")
	before, err := os.ReadFile(otherPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = first.Restart(); err != nil {
		t.Fatal(err)
	}
	if err = first.updateCDISpec(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(otherPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("updating one pool changed another: %v", err)
	}
	assertPrefixReferences(t, registry, merged, name)
	if err = first.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(second.socketPath); err != nil {
		t.Fatalf("stopping one pool removed another: %v", err)
	}
	prefixAllocation(t, conns[1], second.resourceName)
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("unowned legacy endpoint was removed: %v", err)
	}
}

func TestPrefixPoolIdentityAcrossConfigChanges(t *testing.T) {
	registry := prefixCDIRegistry(t)
	first := prefixConfig("a.example", "hca", "ib0")
	other := prefixConfig("b.example", "hca", "ib1")
	devices := prefixDevices(t)
	var previousID, previousAnnotation, previousSpec string
	for index, configs := range [][]*types.UserConfig{{first}, {first, other}, {other, first}, {first}} {
		rm := prefixManager(t, "", configs, false)
		var rs *resourceServer
		for _, server := range rm.resourceServers {
			candidate := server.(*resourceServer)
			if candidate.resourceName == "a.example/hca" {
				rs = candidate
			}
		}
		initialized, err := newResourceServer(first, devices[:1], rm.kubeletRootDir, true, true, rs.socketName)
		if err != nil {
			t.Fatal(err)
		}
		rs = initialized.(*resourceServer)
		if err = rs.updateCDISpec(); err != nil {
			t.Fatal(err)
		}
		allocation, err := rs.Allocate(context.Background(), &pluginapi.AllocateRequest{
			ContainerRequests: []*pluginapi.ContainerAllocateRequest{{DevicesIds: []string{"0"}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		var annotation string
		for key := range allocation.ContainerResponses[0].Annotations {
			annotation = key
		}
		spec := filepath.Join(registry.GetSpecDirectories()[1], "nvidia.com_"+rs.cdiResourceName+".yaml")
		if index > 0 && (rs.cdiResourceName != previousID || annotation != previousAnnotation || spec != previousSpec) {
			t.Fatal("adding, reordering or removing another pool changed persistent identity")
		}
		previousID, previousAnnotation, previousSpec = rs.cdiResourceName, annotation, spec
	}
	entries, err := os.ReadDir(registry.GetSpecDirectories()[1])
	if err != nil || len(entries) != 1 {
		t.Fatalf("restart created duplicate CDI specs: %v, %v", entries, err)
	}
	if err = registry.Refresh(); err != nil {
		t.Fatal(err)
	}
}

func TestPrefixPoolSocketPathCompatibility(t *testing.T) {
	root, err := os.MkdirTemp("", "rdma-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	// This root leaves room for the old short basename, but not a full hash.
	longRoot := filepath.Join(root, strings.Repeat("x", 75-len(root)-len(kubeletPluginRegistry)))
	for _, configs := range [][]*types.UserConfig{
		{prefixConfig("rdma", "hca", "ib0")},
		{prefixConfig("a.example", "hca", "ib0"), prefixConfig("b.example", "hca", "ib1")},
		{prefixConfig("a.example", strings.Repeat("a", 63), "ib0")},
	} {
		rm := prefixManager(t, longRoot, configs, false)
		if len(configs) == 1 && configs[0].ResourceName == "hca" {
			if rs := rm.resourceServers[0].(*resourceServer); rs.socketName != "hca.sock" {
				t.Fatalf("existing short endpoint changed: %s", rs.socketName)
			}
		}
		for _, server := range rm.resourceServers {
			if err = server.Start(); err != nil {
				t.Fatal(err)
			}
		}
		for _, server := range rm.resourceServers {
			if err = server.Stop(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestPrefixPoolSocketBudgetAndCollision(t *testing.T) {
	configs := []*types.UserConfig{prefixConfig("a.example", "hca", "ib0"), prefixConfig("b.example", "hca", "ib1")}
	if _, err := resourceSocketNames(configs, "/"+strings.Repeat("x", 100)); err == nil {
		t.Fatal("unusable socket path accepted")
	}
	// One hash character is deliberately available. Find a genuine collision
	// and verify it is rejected instead of allowing two resources to bind it.
	prefixes := make(map[byte]string)
	for index := range 33 {
		prefix := fmt.Sprintf("p%d.example", index)
		initial := resourcePoolID(prefix + "/hca")[len("pool-")]
		if previous, exists := prefixes[initial]; exists {
			configs[0].ResourcePrefix, configs[1].ResourcePrefix = previous, prefix
			if _, err := resourceSocketNames(configs, "/"+strings.Repeat("x", 94)); err == nil {
				t.Fatal("colliding shortened socket names accepted")
			}
			return
		}
		prefixes[initial] = prefix
	}
	t.Fatal("expected a collision in the 32-character alphabet")
}

func TestPrefixPoolDefaultRootSocketNames(t *testing.T) {
	configs := []*types.UserConfig{prefixConfig("a.example", "hca", "ib0"), prefixConfig("b.example", "hca", "ib1")}
	for _, dir := range []string{getActiveSockDir(defaultKubeletRootDir), getDeprecatedSockDir()} {
		names, err := resourceSocketNames(configs, dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			if len(filepath.Join(dir, name)) > 107 {
				t.Fatalf("default socket path is too long: %s", name)
			}
		}
	}
}
