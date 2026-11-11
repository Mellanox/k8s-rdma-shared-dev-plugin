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
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type recoveryServerPort struct {
	resourcesServerPort
	creates atomic.Int32
	onStop  func()
}

func (p *recoveryServerPort) CreateServer() {
	p.creates.Add(1)
	p.resourcesServerPort.CreateServer()
}

func (p *recoveryServerPort) Stop() {
	if p.onStop != nil {
		p.onStop()
	}
	p.resourcesServerPort.Stop()
}

type recoveryRegistrationServer struct {
	pluginapi.UnimplementedRegistrationServer
	calls atomic.Int32
}

func (r *recoveryRegistrationServer) Register(context.Context, *pluginapi.RegisterRequest) (*pluginapi.Empty, error) {
	r.calls.Add(1)
	return &pluginapi.Empty{}, nil
}

func newRecoveryServer(t *testing.T, watchMode bool) (*resourceServer, *recoveryServerPort) {
	t.Helper()
	dir, err := os.MkdirTemp("", "rdma-recovery-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	port := &recoveryServerPort{}
	rs := &resourceServer{
		resourceName: "rdma/recovery", socketName: "recovery.sock",
		socketPath: filepath.Join(dir, "recovery.sock"), deprecatedSockDir: dir,
		watchMode: watchMode, rsConnector: port,
		stopWatcher: make(chan bool, 1), updateResource: make(chan bool, 1),
		devs: []*pluginapi.Device{{ID: "0", Health: pluginapi.Healthy}},
	}
	if !watchMode {
		listener, listenErr := net.Listen("unix", filepath.Join(dir, kubeEndPoint))
		if listenErr != nil {
			t.Fatal(listenErr)
		}
		registration := &recoveryRegistrationServer{}
		server := grpc.NewServer()
		pluginapi.RegisterRegistrationServer(server, registration)
		go func() { _ = server.Serve(listener) }()
		t.Cleanup(server.Stop)
		t.Cleanup(func() {
			if registration.calls.Load() != port.creates.Load() {
				t.Errorf("registration incomplete: calls=%d starts=%d", registration.calls.Load(), port.creates.Load())
			}
		})
	}
	if err = rs.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if stopErr := rs.Stop(); stopErr != nil {
			t.Error(stopErr)
		}
	})
	return rs, port
}

func recoveryGeneration(rs *resourceServer) uint64 {
	rs.lifecycleMu.Lock()
	defer rs.lifecycleMu.Unlock()
	return rs.generation
}

// A successful Recv verifies that the new endpoint actually serves devices;
// merely observing CreateServer or a self-dial would not prove recovery.
func openRecoveryStream(t *testing.T, rs *resourceServer) (
	pluginapi.DevicePlugin_ListAndWatchClient, context.CancelFunc,
) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	conn, err := grpc.NewClient("unix://"+rs.socketPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	stream, err := pluginapi.NewDevicePluginClient(conn).ListAndWatch(ctx, &pluginapi.Empty{})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	response, err := stream.Recv()
	if err != nil || len(response.GetDevices()) != 1 {
		cancel()
		t.Fatalf("endpoint did not report devices: response=%v err=%v", response, err)
	}
	t.Cleanup(cancel)
	return stream, cancel
}

func awaitRecovery(t *testing.T, rs *resourceServer, generation uint64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for recoveryGeneration(rs) == generation {
		if time.Now().After(deadline) {
			t.Fatal("ListAndWatch cancellation did not recover the endpoint")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestListAndWatchRecovery(t *testing.T) {
	for _, watchMode := range []bool{true, false} {
		t.Run(fmt.Sprintf("watchMode=%t", watchMode), func(t *testing.T) {
			rs, port := newRecoveryServer(t, watchMode)
			generation := recoveryGeneration(rs)
			_, cancel := openRecoveryStream(t, rs)
			cancel()
			awaitRecovery(t, rs, generation)
			openRecoveryStream(t, rs)
			if port.creates.Load() != 2 {
				t.Fatalf("starts=%d, want exactly 2", port.creates.Load())
			}
		})
	}
}

func TestListAndWatchRecoveryDuringIntentionalRestart(t *testing.T) {
	rs, port := newRecoveryServer(t, true)
	generation := recoveryGeneration(rs)
	stream, _ := openRecoveryStream(t, rs)
	if err := rs.Restart(); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); err == nil {
		t.Fatal("intentional Restart did not close the old stream")
	}
	// Simulate a delayed handler entering on the old server after replacement.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	oldInstance := &resourceServerInstance{rs, generation}
	if err := oldInstance.ListAndWatch(nil, &devPluginListAndWatchServerMock{ctx: ctx}); err != nil {
		t.Fatal(err)
	}
	// Also process a late old-instance recovery request synchronously.
	if err := rs.restartIfCurrent(generation); err != nil {
		t.Fatal(err)
	}
	openRecoveryStream(t, rs)
	if port.creates.Load() != 2 {
		t.Fatalf("intentional restart caused duplicate starts: %d", port.creates.Load())
	}
}

func TestListAndWatchRecoveryDuringIntentionalStop(t *testing.T) {
	rs, port := newRecoveryServer(t, true)
	generation := recoveryGeneration(rs)
	stream, _ := openRecoveryStream(t, rs)
	if err := rs.Stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := stream.Recv(); err == nil {
		t.Fatal("Stop did not close the active stream")
	}
	if err := rs.restartIfCurrent(generation); err != nil {
		t.Fatal(err)
	}
	if port.GetServer() != nil || port.creates.Load() != 1 {
		t.Fatal("stream cancellation restarted an intentionally stopped server")
	}
}

func TestListAndWatchRecoveryDeduplicatesRequests(t *testing.T) {
	rs, port := newRecoveryServer(t, true)
	generation := recoveryGeneration(rs)
	var requests sync.WaitGroup
	for range 16 {
		requests.Go(func() {
			if err := rs.restartIfCurrent(generation); err != nil {
				t.Error(err)
			}
		})
	}
	requests.Wait()
	openRecoveryStream(t, rs)
	if port.creates.Load() != 2 {
		t.Fatalf("duplicate recovery requests caused %d starts", port.creates.Load())
	}
}

// Keep lifecycle interleavings independent of network I/O and registration.
type lifecycleRecoveryPort struct{ recoveryServerPort }

func (*lifecycleRecoveryPort) Listen(string, string) (net.Listener, error) { return nil, nil }
func (*lifecycleRecoveryPort) Serve(net.Listener)                          {}
func (*lifecycleRecoveryPort) GetClientConn(string) (*grpc.ClientConn, error) {
	return nil, nil
}
func (*lifecycleRecoveryPort) Close(*grpc.ClientConn) {}

func TestListAndWatchRecoveryAndStop(t *testing.T) {
	for _, recoveryFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("recoveryFirst=%t", recoveryFirst), func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			port := &lifecycleRecoveryPort{}
			port.onStop = func() { once.Do(func() { close(entered); <-release }) }
			rs := &resourceServer{
				socketPath: filepath.Join(t.TempDir(), "recovery.sock"),
				watchMode:  true, rsConnector: port,
			}
			if err := rs.Start(); err != nil {
				t.Fatal(err)
			}
			generation := recoveryGeneration(rs)
			first, second := rs.Stop, func() error { return rs.restartIfCurrent(generation) }
			if recoveryFirst {
				first, second = second, first
			}
			firstDone, secondDone := make(chan error, 1), make(chan error, 1)
			go func() { firstDone <- first() }()
			<-entered
			if rs.lifecycleMu.TryLock() {
				rs.lifecycleMu.Unlock()
				t.Error("in-progress lifecycle transition did not retain the lock")
			}
			secondStarted := make(chan struct{})
			go func() { close(secondStarted); secondDone <- second() }()
			<-secondStarted
			close(release)
			awaitLifecycleOperations(t, firstDone, secondDone)
			if err := rs.restartIfCurrent(generation); err != nil {
				t.Fatal(err)
			}
			if !rs.stopping || port.GetServer() != nil {
				t.Fatal("recovery resurrected a stopped server")
			}
			wantStarts := int32(1)
			if recoveryFirst {
				wantStarts = 2
			}
			if port.creates.Load() != wantStarts {
				t.Fatalf("starts=%d, want %d", port.creates.Load(), wantStarts)
			}
		})
	}
}

func awaitLifecycleOperations(t *testing.T, operations ...<-chan error) {
	t.Helper()
	for _, done := range operations {
		select {
		case err := <-done:
			if err != nil {
				t.Fatal(err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("lifecycle operation deadlocked")
		}
	}
}

// A send error must recover even when Context.Done has not fired yet.
type recoveryFailingStream struct {
	grpc.ServerStream
	sends  int
	failAt int
}

func (*recoveryFailingStream) Context() context.Context { return context.Background() }
func (s *recoveryFailingStream) Send(*pluginapi.ListAndWatchResponse) error {
	s.sends++
	if s.sends == s.failAt {
		return fmt.Errorf("transport closed")
	}
	return nil
}
func TestRecoveryAfterSendFailure(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(fmt.Sprint(failAt), func(t *testing.T) {
			rs, _ := newRecoveryServer(t, true)
			generation := recoveryGeneration(rs)
			rs.updateResource <- true
			instance := &resourceServerInstance{rs, generation}
			if err := instance.ListAndWatch(nil, &recoveryFailingStream{failAt: failAt}); err == nil {
				t.Fatal("expected send error")
			}
			awaitRecovery(t, rs, generation)
			openRecoveryStream(t, rs)
		})
	}
}
func TestRecoveryPreservesReplacementSocket(t *testing.T) {
	old, port := newRecoveryServer(t, true)
	generation := recoveryGeneration(old)
	replacement := &resourceServer{socketPath: old.socketPath, socketName: old.socketName, watchMode: true,
		rsConnector: &resourcesServerPort{}, devs: old.devs, updateResource: make(chan bool, 1)}
	if err := replacement.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = replacement.Stop() })
	before, err := os.Lstat(old.socketPath)
	if err != nil {
		t.Fatal(err)
	}
	if err = old.restartIfCurrent(generation); err != nil {
		t.Fatal(err)
	}
	after, err := os.Lstat(old.socketPath)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(before, after) || port.creates.Load() != 1 {
		t.Fatal("old instance reclaimed replacement socket")
	}
	openRecoveryStream(t, replacement)
}
func TestRecoveryAfterSocketRemoval(t *testing.T) {
	rs, port := newRecoveryServer(t, true)
	generation := recoveryGeneration(rs)
	if err := os.Remove(rs.socketPath); err != nil {
		t.Fatal(err)
	}
	if err := rs.restartIfCurrent(generation); err != nil {
		t.Fatal(err)
	}
	openRecoveryStream(t, rs)
	if port.creates.Load() != 2 {
		t.Fatal("missing socket did not recover")
	}
}
