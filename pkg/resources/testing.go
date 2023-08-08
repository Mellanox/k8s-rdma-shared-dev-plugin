package resources

import (
	"context"
	"fmt"
	"net"
	"path"
	"time"

	"github.com/pkg/errors"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
	registerapi "k8s.io/kubelet/pkg/apis/pluginregistration/v1"
)

// Implementation of pluginapi.RegistrationServer for use in tests.
type fakeRegistrationServer struct {
	grpcServer      *grpc.Server
	sockDir         string
	pluginEndpoint  string
	failOnRegister  bool
	pluginWatchMode bool
}

func createFakeRegistrationServer(sockDir, endpoint string, failOnRegister,
	pluginWatchMode bool) *fakeRegistrationServer {
	return &fakeRegistrationServer{
		sockDir:         sockDir,
		pluginEndpoint:  endpoint,
		failOnRegister:  failOnRegister,
		pluginWatchMode: pluginWatchMode,
	}
}

func (s *fakeRegistrationServer) dial() (registerapi.RegistrationClient, *grpc.ClientConn, error) {
	sockPath := path.Join(s.sockDir, s.pluginEndpoint)
	timeout := 10 * time.Second

	c, err := dial(sockPath, timeout)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to dial socket %s, err: %v", sockPath, err)
	}

	return registerapi.NewRegistrationClient(c), c, nil
}

func dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	var c *grpc.ClientConn
	var err error
	connChannel := make(chan interface{})

	ctx, timeoutCancel := context.WithTimeout(context.TODO(), timeout)
	defer timeoutCancel()
	go func() {
		c, err = grpc.DialContext(ctx, unixSocketPath, grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
				return net.Dial("unix", addr)
			}),
		)
		connChannel <- "done"
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("timout while trying to connect %s", unixSocketPath)

	case <-connChannel:
		return c, err
	}
}

func (s *fakeRegistrationServer) getInfo(ctx context.Context,
	client registerapi.RegistrationClient) (*registerapi.PluginInfo, error) {
	infoResp, err := client.GetInfo(ctx, &registerapi.InfoRequest{})
	if err != nil {
		return infoResp, fmt.Errorf("failed to get plugin info using RPC GetInfo, err: %v", err)
	}

	return infoResp, nil
}

func (s *fakeRegistrationServer) notifyPlugin(client registerapi.RegistrationClient, registered bool,
	errStr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), watchWaitTime)
	defer cancel()

	status := &registerapi.RegistrationStatus{
		PluginRegistered: registered,
		Error:            errStr,
	}

	if _, err := client.NotifyRegistrationStatus(ctx, status); err != nil {
		return errors.Wrap(err, errStr)
	}

	if errStr != "" {
		return errors.New(errStr)
	}

	return nil
}

func (s *fakeRegistrationServer) registerPlugin() error {
	client, conn, err := s.dial()
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = s.getInfo(ctx, client)
	if err != nil {
		return err
	}

	err = s.notifyPlugin(client, true, "")
	if err != nil {
		return err
	}

	if s.failOnRegister {
		return fmt.Errorf("fake registering error")
	}

	return nil
}

func (s *fakeRegistrationServer) Register(ctx context.Context, r *pluginapi.RegisterRequest) (api *pluginapi.Empty,
	err error) {
	s.pluginEndpoint = r.Endpoint
	api = &pluginapi.Empty{}
	if s.failOnRegister {
		err = fmt.Errorf("fake registering error")
	}
	return
}

func (s *fakeRegistrationServer) start() {
	l, err := net.Listen("unix", path.Join(s.sockDir, kubeEndPoint))
	if err != nil {
		panic(err)
	}
	s.grpcServer = grpc.NewServer()
	pluginapi.RegisterRegistrationServer(s.grpcServer, s)
	go func() {
		_ = s.grpcServer.Serve(l)
	}()
	_ = s.waitForServer(watchWaitTime)
}

func (s *fakeRegistrationServer) waitForServer(timeout time.Duration) error {
	maxWaitTime := time.Now().Add(timeout)
	for {
		if time.Now().After(maxWaitTime) {
			return fmt.Errorf("waiting for the fake registration server timed out")
		}
		c, err := net.DialTimeout("unix", path.Join(s.sockDir, kubeEndPoint), time.Second)
		if err == nil {
			return c.Close()
		}
	}
}

func (s *fakeRegistrationServer) stop() {
	s.grpcServer.Stop()
}
