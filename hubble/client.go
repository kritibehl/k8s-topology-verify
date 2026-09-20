package hubble

import (
	"context"
	"fmt"

	observerpb "github.com/cilium/cilium/api/v1/observer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Status struct {
	Version             string
	NumFlows            uint64
	MaxFlows            uint64
	SeenFlows           uint64
	ConnectedNodes      uint32
	UnavailableNodes    uint32
	UnavailableNodeList []string
	FlowsPerSecond      float64
}

type Client struct {
	conn     *grpc.ClientConn
	observer observerpb.ObserverClient
}

func Dial(
	ctx context.Context,
	address string,
) (*Client, error) {
	if address == "" {
		return nil, fmt.Errorf(
			"hubble relay address must not be empty",
		)
	}

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(
			insecure.NewCredentials(),
		),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create hubble grpc client: %w",
			err,
		)
	}

	return &Client{
		conn:     conn,
		observer: observerpb.NewObserverClient(conn),
	}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}

	return c.conn.Close()
}

func (c *Client) ServerStatus(
	ctx context.Context,
) (Status, error) {
	response, err := c.observer.ServerStatus(
		ctx,
		&observerpb.ServerStatusRequest{},
	)
	if err != nil {
		return Status{}, fmt.Errorf(
			"hubble server status: %w",
			err,
		)
	}

	status := Status{
		Version:             response.GetVersion(),
		NumFlows:            response.GetNumFlows(),
		MaxFlows:            response.GetMaxFlows(),
		SeenFlows:           response.GetSeenFlows(),
		UnavailableNodeList: response.GetUnavailableNodes(),
		FlowsPerSecond:      response.GetFlowsRate(),
	}

	if nodes := response.GetNumConnectedNodes(); nodes != nil {
		status.ConnectedNodes = nodes.GetValue()
	}

	if nodes := response.GetNumUnavailableNodes(); nodes != nil {
		status.UnavailableNodes = nodes.GetValue()
	}

	return status, nil
}
