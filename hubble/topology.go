package hubble

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/kritibehl/k8s-topology-verify/topology"

	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type TopologyCollection struct {
	Edges      []topology.ObservedEdge
	FlowEvents uint64
	LostEvents uint64
}

func (c *Client) CollectObservedEdges(
	ctx context.Context,
	sourceSelector string,
	since time.Time,
	until time.Time,
) (TopologyCollection, error) {
	if c == nil || c.observer == nil {
		return TopologyCollection{}, fmt.Errorf(
			"hubble client is not initialized",
		)
	}

	sourceSelector = strings.TrimSpace(
		sourceSelector,
	)

	if sourceSelector == "" {
		return TopologyCollection{}, fmt.Errorf(
			"source label selector must not be empty",
		)
	}

	if !until.After(since) {
		return TopologyCollection{}, fmt.Errorf(
			"until must be after since",
		)
	}

	sinceTimestamp := timestamppb.New(since)

	if err := sinceTimestamp.CheckValid(); err != nil {
		return TopologyCollection{}, fmt.Errorf(
			"invalid since timestamp: %w",
			err,
		)
	}

	untilTimestamp := timestamppb.New(until)

	if err := untilTimestamp.CheckValid(); err != nil {
		return TopologyCollection{}, fmt.Errorf(
			"invalid until timestamp: %w",
			err,
		)
	}

	request := &observerpb.GetFlowsRequest{
		Since: sinceTimestamp,
		Until: untilTimestamp,

		// Topology is directional. Unlike the statistical
		// cohort collector, this intentionally selects
		// flows where the cohort is the source only.
		Whitelist: []*flowpb.FlowFilter{
			{
				SourceLabel: []string{
					sourceSelector,
				},
			},
		},
	}

	stream, err := c.observer.GetFlows(
		ctx,
		request,
	)

	if err != nil {
		return TopologyCollection{}, fmt.Errorf(
			"hubble get topology flows: %w",
			err,
		)
	}

	var collection TopologyCollection

	for {
		response, err := stream.Recv()

		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return TopologyCollection{}, fmt.Errorf(
				"receive hubble topology flow: %w",
				err,
			)
		}

		if lost := response.GetLostEvents(); lost != nil {
			collection.LostEvents +=
				lost.GetNumEventsLost()

			continue
		}

		flow := response.GetFlow()

		if flow == nil {
			continue
		}

		collection.FlowEvents++

		collection.Edges = append(
			collection.Edges,
			observedEdgeFromFlow(flow),
		)
	}

	return collection, nil
}

func observedEdgeFromFlow(
	flow *flowpb.Flow,
) topology.ObservedEdge {
	if flow == nil {
		return topology.ObservedEdge{}
	}

	source := flow.GetSource()
	destination := flow.GetDestination()
	ip := flow.GetIP()
	service := flow.GetDestinationService()

	return topology.ObservedEdge{
		SourceNamespace: source.GetNamespace(),
		SourcePod:       source.GetPodName(),
		SourceIP:        ip.GetSource(),
		SourceLabels: append(
			[]string(nil),
			source.GetLabels()...,
		),

		DestinationNamespace: destination.GetNamespace(),

		DestinationPod: destination.GetPodName(),

		DestinationIP: ip.GetDestination(),

		DestinationLabels: append(
			[]string(nil),
			destination.GetLabels()...,
		),

		DestinationService: namespacedService(
			service.GetNamespace(),
			service.GetName(),
		),

		Verdict: flow.GetVerdict().String(),
	}
}

func namespacedService(
	namespace string,
	name string,
) string {
	if name == "" {
		return ""
	}

	if namespace == "" {
		return name
	}

	return namespace + "/" + name
}
