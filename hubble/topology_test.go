package hubble

import (
	"context"
	"net"
	"testing"
	"time"

	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"google.golang.org/grpc"
)

type topologyObserver struct {
	observerpb.UnimplementedObserverServer

	request *observerpb.GetFlowsRequest
}

func (o *topologyObserver) GetFlows(
	request *observerpb.GetFlowsRequest,
	stream grpc.ServerStreamingServer[observerpb.GetFlowsResponse],
) error {
	o.request = request

	responses := []*observerpb.GetFlowsResponse{
		flowResponse(
			&flowpb.Flow{
				Verdict: flowpb.Verdict_FORWARDED,

				IP: &flowpb.IP{
					Source:      "10.244.0.10",
					Destination: "10.244.0.20",
				},

				Source: &flowpb.Endpoint{
					Namespace: "shop",
					PodName:   "checkout-v2",
					Labels: []string{
						"k8s:app=checkout",
						"k8s:version=v2",
					},
				},

				Destination: &flowpb.Endpoint{
					Namespace: "shop",
					PodName:   "payment-v2-a",
					Labels: []string{
						"k8s:app=payment",
						"k8s:version=v2",
					},
				},

				DestinationService: &flowpb.Service{
					Namespace: "shop",
					Name:      "payment-canary",
				},
			},
		),

		flowResponse(
			&flowpb.Flow{
				Verdict: flowpb.Verdict_DROPPED,

				IP: &flowpb.IP{
					Source:      "10.244.0.10",
					Destination: "10.244.0.30",
				},

				Source: &flowpb.Endpoint{
					Namespace: "shop",
					PodName:   "checkout-v2",
				},

				Destination: &flowpb.Endpoint{
					Namespace: "shop",
					PodName:   "payment-v1",
				},
			},
		),

		{
			ResponseTypes: &observerpb.GetFlowsResponse_LostEvents{
				LostEvents: &flowpb.LostEvent{
					NumEventsLost: 3,
				},
			},
		},
	}

	for _, response := range responses {
		if err := stream.Send(response); err != nil {
			return err
		}
	}

	return nil
}

func TestCollectObservedEdgesOverGRPC(
	t *testing.T,
) {
	listener, err := net.Listen(
		"tcp",
		"127.0.0.1:0",
	)

	if err != nil {
		t.Fatal(err)
	}

	defer listener.Close()

	observer := &topologyObserver{}

	server := grpc.NewServer()

	observerpb.RegisterObserverServer(
		server,
		observer,
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

	until := time.Now().UTC()
	since := until.Add(-30 * time.Second)

	collection, err := client.CollectObservedEdges(
		ctx,
		"k8s:app=checkout,k8s:version=v2",
		since,
		until,
	)

	if err != nil {
		t.Fatal(err)
	}

	if observer.request == nil {
		t.Fatal(
			"expected Hubble GetFlows request",
		)
	}

	if len(observer.request.GetWhitelist()) != 1 {
		t.Fatalf(
			"expected exactly 1 directional whitelist filter, got %d",
			len(observer.request.GetWhitelist()),
		)
	}

	filter := observer.request.GetWhitelist()[0]

	if len(filter.GetSourceLabel()) != 1 {
		t.Fatalf(
			"expected source-label filter: %+v",
			filter.GetSourceLabel(),
		)
	}

	if len(filter.GetDestinationLabel()) != 0 {
		t.Fatalf(
			"topology collection must not use destination-label cohort filtering: %+v",
			filter.GetDestinationLabel(),
		)
	}

	if collection.FlowEvents != 2 {
		t.Fatalf(
			"expected 2 flow events, got %d",
			collection.FlowEvents,
		)
	}

	if collection.LostEvents != 3 {
		t.Fatalf(
			"expected 3 lost events, got %d",
			collection.LostEvents,
		)
	}

	if len(collection.Edges) != 2 {
		t.Fatalf(
			"expected 2 observed edges, got %d",
			len(collection.Edges),
		)
	}

	edge := collection.Edges[0]

	if edge.SourceNamespace != "shop" ||
		edge.SourcePod != "checkout-v2" ||
		edge.SourceIP != "10.244.0.10" {
		t.Fatalf(
			"unexpected source identity: %+v",
			edge,
		)
	}

	if edge.DestinationNamespace != "shop" ||
		edge.DestinationPod != "payment-v2-a" ||
		edge.DestinationIP != "10.244.0.20" {
		t.Fatalf(
			"unexpected destination identity: %+v",
			edge,
		)
	}

	if edge.DestinationService !=
		"shop/payment-canary" {
		t.Fatalf(
			"unexpected destination service %q",
			edge.DestinationService,
		)
	}

	if edge.Verdict != "FORWARDED" {
		t.Fatalf(
			"expected FORWARDED verdict, got %q",
			edge.Verdict,
		)
	}

	if len(edge.SourceLabels) != 2 {
		t.Fatalf(
			"expected source labels, got %+v",
			edge.SourceLabels,
		)
	}

	if len(edge.DestinationLabels) != 2 {
		t.Fatalf(
			"expected destination labels, got %+v",
			edge.DestinationLabels,
		)
	}

	if collection.Edges[1].Verdict != "DROPPED" {
		t.Fatalf(
			"expected second edge to preserve DROPPED verdict, got %+v",
			collection.Edges[1],
		)
	}
}
