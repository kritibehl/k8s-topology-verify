package hubble

import (
	"context"
	"net"
	"testing"
	"time"

	observerpb "github.com/cilium/cilium/api/v1/observer"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type fakeObserver struct {
	observerpb.UnimplementedObserverServer
}

func (fakeObserver) ServerStatus(
	ctx context.Context,
	request *observerpb.ServerStatusRequest,
) (*observerpb.ServerStatusResponse, error) {
	return &observerpb.ServerStatusResponse{
		Version:             "1.20.0-test",
		NumFlows:            800,
		MaxFlows:            4096,
		SeenFlows:           12345,
		NumConnectedNodes:   wrapperspb.UInt32(3),
		NumUnavailableNodes: wrapperspb.UInt32(1),
		UnavailableNodes:    []string{"node-bad"},
		FlowsRate:           42.5,
	}, nil
}

func TestServerStatusOverGRPC(t *testing.T) {
	listener, err := net.Listen(
		"tcp",
		"127.0.0.1:0",
	)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	server := grpc.NewServer()

	observerpb.RegisterObserverServer(
		server,
		fakeObserver{},
	)

	go func() {
		_ = server.Serve(listener)
	}()

	defer server.Stop()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	client, err := Dial(
		ctx,
		listener.Addr().String(),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	status, err := client.ServerStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if status.Version != "1.20.0-test" {
		t.Fatalf(
			"expected version 1.20.0-test, got %q",
			status.Version,
		)
	}

	if status.ConnectedNodes != 3 {
		t.Fatalf(
			"expected 3 connected nodes, got %d",
			status.ConnectedNodes,
		)
	}

	if status.UnavailableNodes != 1 {
		t.Fatalf(
			"expected 1 unavailable node, got %d",
			status.UnavailableNodes,
		)
	}

	if len(status.UnavailableNodeList) != 1 ||
		status.UnavailableNodeList[0] != "node-bad" {
		t.Fatalf(
			"unexpected unavailable nodes: %+v",
			status.UnavailableNodeList,
		)
	}

	if status.FlowsPerSecond != 42.5 {
		t.Fatalf(
			"expected 42.5 flows/s, got %f",
			status.FlowsPerSecond,
		)
	}
}
