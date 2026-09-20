package topology

type Decision string

const (
	DecisionPass         Decision = "PASS"
	DecisionFail         Decision = "FAIL"
	DecisionInconclusive Decision = "INCONCLUSIVE"
)

const (
	ReasonControlDataPlaneDivergence = "CONTROL_DATA_PLANE_DIVERGENCE"

	ReasonNoExpectedEndpoints = "NO_EXPECTED_ENDPOINTS"

	ReasonNoObservedTraffic = "NO_OBSERVED_FORWARDED_TRAFFIC"
)

type ExpectedEndpoint struct {
	Namespace string            `json:"namespace"`
	Pod       string            `json:"pod,omitempty"`
	IP        string            `json:"ip,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
}

type ObservedEdge struct {
	SourceNamespace string   `json:"sourceNamespace,omitempty"`
	SourcePod       string   `json:"sourcePod,omitempty"`
	SourceIP        string   `json:"sourceIP,omitempty"`
	SourceLabels    []string `json:"sourceLabels,omitempty"`

	DestinationNamespace string   `json:"destinationNamespace,omitempty"`
	DestinationPod       string   `json:"destinationPod,omitempty"`
	DestinationIP        string   `json:"destinationIP,omitempty"`
	DestinationLabels    []string `json:"destinationLabels,omitempty"`
	DestinationService   string   `json:"destinationService,omitempty"`

	Verdict string `json:"verdict,omitempty"`
}

type Report struct {
	Decision Decision `json:"decision"`
	Reason   string   `json:"reason,omitempty"`

	ExpectedEndpoints []ExpectedEndpoint `json:"expectedEndpoints"`
	ObservedEdges     []ObservedEdge     `json:"observedEdges"`
	UnexpectedEdges   []ObservedEdge     `json:"unexpectedEdges"`

	Reasons []string `json:"reasons"`
}
