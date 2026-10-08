// Copyright 2026 xinjun.jiang
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
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"

	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types"
	"github.com/Mellanox/k8s-rdma-shared-dev-plugin/pkg/types/mocks"
)

func TestUpdateDevicesWithoutListAndWatch(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		device := mocks.NewMockPciNetDevice(t)
		device.On("GetRdmaSpec").Return([]*pluginapi.DeviceSpec{{HostPath: "/dev/infiniband/uverbs0"}})
		devices := []types.PciNetDevice{device}
		server, err := newResourceServer(&types.UserConfig{
			ResourceName: "test", ResourcePrefix: "rdma", RdmaHcaMax: 2,
		}, nil, t.TempDir(), true, false)
		if err != nil {
			t.Fatal(err)
		}
		rs := server.(*resourceServer)
		rs.UpdateDevices(devices)

		done := make(chan struct{})
		go func() {
			rs.UpdateDevices(nil)
			rs.UpdateDevices(devices)
			close(done)
		}()
		synctest.Wait()
		select {
		case <-done:
		default:
			// Release the blocked sender before leaving the synctest bubble.
			<-rs.updateResource
			synctest.Wait()
			<-rs.updateResource
			synctest.Wait()
			t.Fatal("device discovery blocked while no ListAndWatch stream was connected")
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		stream := &devPluginListAndWatchServerMock{ctx: ctx}
		if err = rs.ListAndWatch(nil, stream); err != nil {
			t.Fatal(err)
		}
		if len(stream.devices) != 2 {
			t.Fatalf("new ListAndWatch stream received %d devices, want 2", len(stream.devices))
		}
		allocation, err := rs.Allocate(context.Background(), &pluginapi.AllocateRequest{
			ContainerRequests: []*pluginapi.ContainerAllocateRequest{{DevicesIDs: []string{"0"}}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := allocation.ContainerResponses[0].Devices; len(got) != 1 || got[0].HostPath != "/dev/infiniband/uverbs0" {
			t.Fatalf("allocation did not use the latest device inventory: %v", got)
		}
	})
}

func TestListAndWatchSendFailureWithPendingUpdate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		rs := &resourceServer{updateResource: make(chan bool, 1)}
		rs.updateResource <- true
		wantErr := errors.New("stream closed")
		stream := &failingUpdateStream{updates: rs.updateResource, err: wantErr}
		done := make(chan error, 1)
		go func() { done <- rs.ListAndWatch(nil, stream) }()
		synctest.Wait()
		select {
		case err := <-done:
			if !errors.Is(err, wantErr) {
				t.Fatalf("ListAndWatch returned %v, want %v", err, wantErr)
			}
		default:
			<-rs.updateResource
			synctest.Wait()
			t.Fatal("failed ListAndWatch stream blocked trying to requeue an already pending update")
		}
		select {
		case <-rs.updateResource:
		default:
			t.Fatal("failed ListAndWatch stream lost the pending update")
		}
	})
}

func TestDeviceUpdatesBeforeWatch(t *testing.T) {
	device := mocks.NewMockPciNetDevice(t)
	device.On("GetRdmaSpec").Return([]*pluginapi.DeviceSpec{{HostPath: "/dev/infiniband/uverbs0"}})
	devices := []types.PciNetDevice{device}
	server, err := newResourceServer(&types.UserConfig{
		ResourceName: "test", ResourcePrefix: "rdma", RdmaHcaMax: 2,
	}, nil, t.TempDir(), true, false)
	if err != nil {
		t.Fatal(err)
	}
	rs := server.(*resourceServer)
	socketDir, err := os.MkdirTemp("", "rdma-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	rs.socketPath = filepath.Join(socketDir, "plugin.sock")
	if err = rs.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if stopErr := rs.Stop(); stopErr != nil {
			t.Error(stopErr)
		}
	})

	done := make(chan struct{})
	go func() {
		rs.UpdateDevices(devices)
		rs.UpdateDevices(nil)
		rs.UpdateDevices(devices)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		// Release both blocked updates before stopping the server.
		<-rs.updateResource
		<-rs.updateResource
		<-done
		t.Fatal("device discovery blocked before the kubelet connected")
	}

	conn, err := grpc.NewClient("unix://"+rs.socketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := pluginapi.NewDevicePluginClient(conn).ListAndWatch(ctx, &pluginapi.Empty{})
	if err != nil {
		t.Fatal(err)
	}
	response, err := stream.Recv()
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Devices) != 2 {
		t.Fatalf("kubelet received %d devices, want the 2 devices in the latest inventory", len(response.Devices))
	}
}

type failingUpdateStream struct {
	grpc.ServerStream
	updates chan bool
	err     error
	sends   int
}

func (s *failingUpdateStream) Context() context.Context {
	return context.Background()
}

func (s *failingUpdateStream) Send(*pluginapi.ListAndWatchResponse) error {
	s.sends++
	if s.sends == 1 {
		return nil
	}
	// Model another producer queuing an update before the failed send is retried.
	s.updates <- true
	return s.err
}
