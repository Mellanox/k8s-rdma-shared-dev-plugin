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
	"crypto/sha256"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	cdiLib "github.com/container-orchestrated-devices/container-device-interface/pkg/cdi"
	cdiSpecs "github.com/container-orchestrated-devices/container-device-interface/specs-go"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
)

func TestResourceManagerCDICleanup(t *testing.T) {
	for _, operation := range []struct {
		name string
		run  func(*resourceManager) error
	}{
		{"initialize", (*resourceManager).InitServers},
		{"stop", (*resourceManager).StopAllServers},
		{"restart", (*resourceManager).RestartAllServers},
	} {
		t.Run(operation.name, func(t *testing.T) {
			dirs := []string{t.TempDir(), t.TempDir()}
			registry := cdiLib.GetRegistry()
			previousDirs := registry.GetSpecDirectories()
			if err := registry.Configure(cdiLib.WithSpecDirs(dirs...)); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := registry.Configure(cdiLib.WithSpecDirs(previousDirs...)); err != nil {
					t.Error(err)
				}
			})

			owned, preserved := createCDICleanupFixtures(t, dirs)
			rm := &resourceManager{
				useCdi: true,
				configList: []*types.UserConfig{{
					ResourceName: "test_pool", ResourcePrefix: "rdma", RdmaHcaMax: 1,
				}},
			}
			if err := operation.run(rm); err != nil {
				t.Fatal(err)
			}
			for _, path := range owned {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Errorf("RDMA spec %s was not removed: %v", path, err)
				} else {
					t.Logf("removed RDMA spec: %s", path)
				}
			}
			for path, before := range preserved {
				after, err := os.ReadFile(path)
				if err != nil {
					t.Error(err)
					continue
				}
				if !bytes.Equal(before, after) {
					t.Errorf("unrelated CDI file %s changed", path)
				}
				t.Logf("preserved %s: before=%x after=%x", path,
					sha256.Sum256(before), sha256.Sum256(after))
			}
		})
	}
}

func createCDICleanupFixtures(t *testing.T, dirs []string) ([]string, map[string][]byte) {
	t.Helper()
	var owned []string
	preserved := make(map[string][]byte)
	for _, dir := range dirs {
		for _, fixture := range []struct {
			name string
			kind string
		}{
			{"nvidia.com_test-pool.yaml", "nvidia.com/net-rdma"},
			{"nvidia.com-gpu.json", "nvidia.com/gpu"},
			{"nvidia.com.example_net.yaml", "nvidia.com.example/net-rdma"},
			{"nvidia.com-invalid.yaml", ""},
		} {
			data, err := json.Marshal(cdiSpecs.Spec{Version: cdiSpecs.CurrentVersion, Kind: fixture.kind})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, fixture.name)
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if fixture.kind == cdiResourcePrefix+"/"+cdiResourceKind {
				owned = append(owned, path)
			} else {
				preserved[path] = data
			}
		}
	}
	return owned, preserved
}
