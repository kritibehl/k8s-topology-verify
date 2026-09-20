package hubble
 
import (
	flowpb "github.com/cilium/cilium/api/v1/flow"
	observerpb "github.com/cilium/cilium/api/v1/observer"
)
 
func flowResponse(
	flow *flowpb.Flow,
) *observerpb.GetFlowsResponse {
	return &observerpb.GetFlowsResponse{
		ResponseTypes: &observerpb.GetFlowsResponse_Flow{
			Flow: flow,
		},
	}
}
