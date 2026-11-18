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
	"github.com/jaypipes/ghw"
	"github.com/jaypipes/pcidb"
	"github.com/stretchr/testify/mock"
	"github.com/vishvananda/netlink"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	registerapi "k8s.io/kubelet/pkg/apis/pluginregistration/v1"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/cdi"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types/mocks"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/utils"
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

func prefixSocketRoot(t *testing.T) string {
	t.Helper()
	// Leave enough room for the 63-character legacy socket fixture even with a long TMPDIR.
	root, err := os.MkdirTemp("/tmp", "rdma-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	return root
}

func prefixManager(t *testing.T, root string, configs []*types.UserConfig, useCDI bool) *resourceManager {
	t.Helper()
	if root == "" {
		root = prefixSocketRoot(t)
	}
	if err := os.MkdirAll(getActiveSockDir(root), 0700); err != nil {
		t.Fatal(err)
	}
	rm := &resourceManager{defaultResourcePrefix: "rdma", kubeletRootDir: root,
		watchMode: true, useCdi: useCDI, configList: configs}
	if useCDI {
		prefixDeviceFixtures(t, rm)
	}
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
	// Refresh synchronously when asserting what the completed writes put on disk.
	if err := registry.Configure(
		cdiLib.WithSpecDirs(t.TempDir(), t.TempDir()), cdiLib.WithAutoRefresh(false),
	); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := registry.Configure(cdiLib.WithSpecDirs(oldDirs...), cdiLib.WithAutoRefresh(true)); err != nil {
			t.Error(err)
		}
	})
	return registry
}

func prefixDeviceFixtures(t *testing.T, rm *resourceManager) {
	t.Helper()
	sysBus := t.TempDir()
	previousSysBus := utils.SysBusPci
	utils.SysBusPci = sysBus
	t.Cleanup(func() { utils.SysBusPci = previousSysBus })

	link := &FakeLink{netlink.LinkAttrs{EncapType: "infiniband"}}
	nLink := mocks.NewMockNetlinkManager(t)
	nLink.On("LinkByName", mock.Anything).Return(link, nil)
	nLink.On("LinkSetUp", link).Return(nil)
	rm.netlinkManager = nLink

	rds := mocks.NewMockRdmaDeviceSpec(t)
	rds.On("VerifyRdmaSpec", mock.Anything).Return(nil)
	rm.rds = rds
	for index := range 2 {
		addr := prefixPCIAddress(index)
		deviceDir := filepath.Join(sysBus, addr)
		if err := os.MkdirAll(filepath.Join(deviceDir, "net", fmt.Sprintf("ib%d", index)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("mlx5_core", filepath.Join(deviceDir, "driver")); err != nil {
			t.Fatal(err)
		}
		rm.deviceList = append(rm.deviceList, &ghw.PCIDevice{
			Address: addr, Vendor: &pcidb.Vendor{ID: "15b3"}, Product: &pcidb.Product{ID: "1017"},
		})
		rds.On("Get", addr).Return(prefixDeviceSpec(index))
	}
}

func prefixPCIAddress(index int) string {
	return fmt.Sprintf("0000:00:00.%d", index+1)
}

func prefixDeviceSpec(index int) []*pluginapi.DeviceSpec {
	node := fmt.Sprintf("/dev/infiniband/uverbs%d", index)
	return []*pluginapi.DeviceSpec{{HostPath: node, ContainerPath: node}}
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

func prefixAllocation(
	t *testing.T, conn *grpc.ClientConn, resourceName string, writer *observedCDIWriter,
) map[string]string {
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
	devices := writer.receiveWrite(ctx, t, stream)
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

func startPrefixCDIServers(t *testing.T, rm *resourceManager) []*observedCDIWriter {
	t.Helper()
	writers := make([]*observedCDIWriter, 0, len(rm.resourceServers))
	for _, server := range rm.resourceServers {
		writers = append(writers, observeCDIWrites(server.(*resourceServer), ""))
	}
	t.Cleanup(func() {
		if err := rm.StopAllServers(); err != nil {
			t.Error(err)
		}
	})
	if err := rm.StartAllServers(); err != nil {
		t.Fatal(err)
	}
	return writers
}

func prefixReference(poolName string, index int) string {
	name := prefixPCIAddress(index)
	if poolName != "" {
		name = poolName + "_" + name
	}
	return cdiLib.QualifiedName(cdiResourcePrefix, cdiResourceKind, name)
}

func assertPrefixAnnotation(t *testing.T, annotations map[string]string, expected string) string {
	t.Helper()
	keys, names, err := cdiLib.ParseAnnotations(annotations)
	if err != nil || len(keys) != 1 || len(names) != 1 || names[0] != expected {
		t.Fatalf("Allocate annotations %v: expected a single reference to %q, got %v (%v)",
			annotations, expected, names, err)
	}
	return keys[0]
}

func assertPrefixReference(t *testing.T, registry cdiLib.Registry, name string, index int) {
	t.Helper()
	device := registry.DeviceDB().GetDevice(name)
	if device == nil {
		t.Fatalf("qualified CDI name %s did not resolve", name)
	}
	nodes := device.ContainerEdits.DeviceNodes
	expected := prefixDeviceSpec(index)[0].HostPath
	if len(nodes) != 1 || nodes[0].HostPath != expected {
		t.Fatalf("CDI reference %s resolved to the wrong pool: %v", name, nodes)
	}
	definitions := 0
	for _, spec := range registry.SpecDB().GetVendorSpecs(cdiResourcePrefix) {
		for _, entry := range spec.Devices {
			if cdiLib.QualifiedName(cdiResourcePrefix, cdiResourceKind, entry.Name) == name {
				definitions++
			}
		}
	}
	if definitions != 1 {
		t.Fatalf("qualified CDI name %s has %d definitions, want one", name, definitions)
	}
}

func assertPrefixPoolReferences(t *testing.T, registry cdiLib.Registry, name string, references map[string]int) {
	t.Helper()
	if err := registry.Refresh(); err != nil {
		t.Fatal(err)
	}
	for reference, index := range references {
		assertPrefixReference(t, registry, reference, index)
		assertPrefixReference(t, registry, prefixReference(name, index), index)
		assertPrefixReference(t, registry, prefixReference("", index), index)
	}
}

func TestPrefixPoolsEndpointsAndCDI(t *testing.T) {
	for _, name := range []string{"hca", strings.Repeat("h", 63)} {
		t.Run(name, func(t *testing.T) { testPrefixPoolsEndpointsAndCDI(t, name, 1) })
	}
	t.Run("overlapping selectors", func(t *testing.T) { testPrefixPoolsEndpointsAndCDI(t, "hca", 0) })
}

func testPrefixPoolsEndpointsAndCDI(t *testing.T, name string, secondDevice int) {
	registry := prefixCDIRegistry(t)
	device := &pciNetDevice{pciAddress: prefixPCIAddress(0), rdmaSpec: prefixDeviceSpec(0)}
	err := cdi.New().CreateCDISpec(
		cdiResourcePrefix, cdiResourceKind, name, []types.PciNetDevice{device}, nil)
	if err != nil {
		t.Fatal(err)
	}
	oldSpec := filepath.Join(registry.GetSpecDirectories()[1], "nvidia.com_"+name+".yaml")
	rm := prefixManager(t, "", []*types.UserConfig{
		prefixConfig("a.example", name, "ib0"), prefixConfig("b.example", name, fmt.Sprintf("ib%d", secondDevice)),
	}, true)
	legacyPath := filepath.Join(getActiveSockDir(rm.kubeletRootDir), name+".sock")
	legacy, listenErr := net.Listen("unix", legacyPath)
	if listenErr != nil {
		t.Fatal(listenErr)
	}
	t.Cleanup(func() { _ = legacy.Close() })
	if _, statErr := os.Stat(oldSpec); !os.IsNotExist(statErr) {
		t.Fatalf("legacy spec survived startup and could conflict with its replacement: %v", err)
	}
	first := rm.resourceServers[0].(*resourceServer)
	second := rm.resourceServers[1].(*resourceServer)
	writers := startPrefixCDIServers(t, rm)
	firstWriter, secondWriter := writers[0], writers[1]
	if first.socketPath == second.socketPath || first.cdiSpecName == second.cdiSpecName {
		t.Fatal("different prefixed pools share an internal identity")
	}
	firstConn, secondConn := prefixConnection(t, first), prefixConnection(t, second)
	firstReference := prefixReference(resourcePoolID(first.resourceName), 0)
	secondReference := prefixReference(resourcePoolID(second.resourceName), secondDevice)
	firstAnnotations := prefixAllocation(t, firstConn, first.resourceName, firstWriter)
	secondAnnotations := prefixAllocation(t, secondConn, second.resourceName, secondWriter)
	firstKey := assertPrefixAnnotation(t, firstAnnotations, firstReference)
	secondKey := assertPrefixAnnotation(t, secondAnnotations, secondReference)
	if firstKey == secondKey {
		t.Fatalf("merging allocations lost a pool: shared annotation key %q", firstKey)
	}
	references := map[string]int{firstReference: 0, secondReference: secondDevice}
	assertPrefixPoolReferences(t, registry, name, references)
	otherPath := filepath.Join(registry.GetSpecDirectories()[1], "nvidia.com_"+second.cdiSpecName+".yaml")
	before, err := os.ReadFile(otherPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = first.Restart(); err != nil {
		t.Fatal(err)
	}
	updatedAnnotations := prefixAllocation(t, prefixConnection(t, first), first.resourceName, firstWriter)
	assertPrefixAnnotation(t, updatedAnnotations, firstReference)
	after, err := os.ReadFile(otherPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("updating one pool changed another: %v", err)
	}
	assertPrefixPoolReferences(t, registry, name, references)
	if err = first.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(second.socketPath); err != nil {
		t.Fatalf("stopping one pool removed another: %v", err)
	}
	prefixAllocation(t, secondConn, second.resourceName, secondWriter)
	if _, err := os.Stat(legacyPath); err != nil {
		t.Fatalf("unowned legacy endpoint was removed: %v", err)
	}
}

type observedCDIWriter struct {
	cdi.CDI
	legacyName string
	written    chan error
}

func (w *observedCDIWriter) CreateCDISpec(
	prefix, kind, name string, devices []types.PciNetDevice, aliases []cdi.DeviceAlias,
) error {
	if w.legacyName != "" {
		// The outgoing plugin writes only pool-qualified and bare PCI names.
		name = w.legacyName
		aliases = make([]cdi.DeviceAlias, 0, len(devices))
		for _, device := range devices {
			aliases = append(aliases, cdi.DeviceAlias{Name: device.GetPciAddr(), Device: device})
		}
	}
	err := w.CDI.CreateCDISpec(prefix, kind, name, devices, aliases)
	w.written <- err
	return err
}

func observeCDIWrites(rs *resourceServer, legacyName string) *observedCDIWriter {
	writer := &observedCDIWriter{CDI: rs.cdi, legacyName: legacyName, written: make(chan error, 3)}
	rs.cdi = writer
	return writer
}

func (w *observedCDIWriter) receiveWrite(
	ctx context.Context, t *testing.T, stream pluginapi.DevicePlugin_ListAndWatchClient,
) *pluginapi.ListAndWatchResponse {
	t.Helper()
	response, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-w.written:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	return response
}

func TestLegacyCDIWriterAcrossReplacement(t *testing.T) {
	const name = "hca"
	registry := prefixCDIRegistry(t)
	specDir := registry.GetSpecDirectories()[1]
	legacySpec := filepath.Join(specDir, "nvidia.com_"+name+".yaml")
	config := prefixConfig("example.com", name, "ib0")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := func(rm *resourceManager, legacyName string) (*resourceServer, *observedCDIWriter) {
		rs := rm.resourceServers[0].(*resourceServer)
		writer := observeCDIWrites(rs, legacyName)
		if err := rs.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := rs.Stop(); err != nil {
				t.Error(err)
			}
		})
		return rs, writer
	}
	watch := func(rs *resourceServer) pluginapi.DevicePlugin_ListAndWatchClient {
		stream, err := pluginapi.NewDevicePluginClient(prefixConnection(t, rs)).ListAndWatch(ctx, &pluginapi.Empty{})
		if err != nil {
			t.Fatal(err)
		}
		return stream
	}

	oldManager := prefixManager(t, "", []*types.UserConfig{config}, true)
	// The outgoing process uses the pre-upgrade filename in its real ListAndWatch RPC.
	old, oldWriter := start(oldManager, name)
	oldStream := watch(old)
	oldWriter.receiveWrite(ctx, t, oldStream)
	if _, err := os.Stat(legacySpec); err != nil {
		t.Fatal(err)
	}

	replacement := prefixManager(t, oldManager.kubeletRootDir, []*types.UserConfig{config}, true)
	if _, err := os.Stat(legacySpec); !os.IsNotExist(err) {
		t.Fatalf("startup did not clean the prior spec: %v", err)
	}
	old.notifyUpdate()
	oldWriter.receiveWrite(ctx, t, oldStream)
	if _, err := os.Stat(legacySpec); err != nil {
		t.Fatalf("the active legacy writer did not recreate its spec after initialization: %v", err)
	}

	current, currentWriter := start(replacement, "")
	if current.socketPath != old.socketPath {
		t.Fatalf("replacement didn't take over the legacy socket: %q", current.socketPath)
	}
	annotations := prefixAllocation(t, prefixConnection(t, current), current.resourceName, currentWriter)
	assertPrefixAnnotation(t, annotations, prefixReference(name, 0))
	// Unlinking the listener does not close the already-established stream.
	old.notifyUpdate()
	oldWriter.receiveWrite(ctx, t, oldStream)

	if err := registry.Refresh(); err != nil {
		t.Fatal(err)
	}
	assertPrefixReference(t, registry, prefixReference(name, 0), 0)
	assertPrefixReference(t, registry, prefixReference("", 0), 0)
	if ref := prefixReference(resourcePoolID(current.resourceName), 0); registry.DeviceDB().GetDevice(ref) != nil {
		t.Fatalf("legacy writer unexpectedly defined %s", ref)
	}
	entries, err := os.ReadDir(specDir)
	if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(legacySpec) {
		t.Fatalf("overlapping writers should share the legacy file, got %v: %v", entries, err)
	}
}

func allocatePrefixPools(t *testing.T, rm *resourceManager, resourceName string) map[string]string {
	t.Helper()
	writers := startPrefixCDIServers(t, rm)
	var selected map[string]string
	for index, server := range rm.resourceServers {
		rs := server.(*resourceServer)
		annotations := prefixAllocation(t, prefixConnection(t, rs), rs.resourceName, writers[index])
		if rs.resourceName == resourceName {
			selected = annotations
		}
	}
	if selected == nil {
		t.Fatalf("configuration lost %s", resourceName)
	}
	return selected
}

func TestPrefixPoolIdentityAcrossConfigChanges(t *testing.T) {
	registry := prefixCDIRegistry(t)
	specDir := registry.GetSpecDirectories()[1]
	first := prefixConfig("a.example", "hca", "ib0")
	other := prefixConfig("b.example", "hca", "ib1")
	root := prefixSocketRoot(t)
	savedReferences := map[string]bool{
		prefixReference(first.ResourceName, 0): true,
		prefixReference("", 0):                 true,
	}
	var previousAnnotation string
	for _, stage := range []struct {
		name    string
		configs []*types.UserConfig
	}{
		{"unique", []*types.UserConfig{first}},
		{"shared", []*types.UserConfig{first, other}},
		{"reordered", []*types.UserConfig{other, first}},
		{"unique again", []*types.UserConfig{first}},
	} {
		// Each subtest stops its spec writers before the next configuration starts.
		t.Run(stage.name, func(t *testing.T) {
			rm := prefixManager(t, root, stage.configs, true)
			const resourceName = "a.example/hca"
			annotations := allocatePrefixPools(t, rm, resourceName)
			expectedSpec := first.ResourceName
			if len(stage.configs) > 1 {
				expectedSpec = resourcePoolID(resourceName)
			}
			reference := prefixReference(expectedSpec, 0)
			key := assertPrefixAnnotation(t, annotations, reference)
			if previousAnnotation != "" && key != previousAnnotation {
				t.Fatalf("annotation key changed from %q to %q", previousAnnotation, key)
			}
			previousAnnotation = key
			savedReferences[reference] = true
			if _, err := os.Stat(filepath.Join(specDir, "nvidia.com_"+expectedSpec+".yaml")); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(specDir)
			if err != nil || len(entries) != len(stage.configs) {
				t.Fatalf("restart did not leave exactly one CDI spec per pool: %v, %v", entries, err)
			}
			if err := registry.Refresh(); err != nil {
				t.Fatal(err)
			}
			for reference := range savedReferences {
				assertPrefixReference(t, registry, reference, 0)
			}
		})
	}
}

func TestPrefixPoolSocketPathCompatibility(t *testing.T) {
	root := prefixSocketRoot(t)
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
			if err := server.Start(); err != nil {
				t.Fatal(err)
			}
		}
		for _, server := range rm.resourceServers {
			if err := server.Stop(); err != nil {
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
