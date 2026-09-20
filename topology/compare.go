package topology

import (
	"fmt"
	"sort"
	"strings"
)

func Compare(
	expected []ExpectedEndpoint,
	observed []ObservedEdge,
) Report {
	report := Report{
		ExpectedEndpoints: append(
			[]ExpectedEndpoint(nil),
			expected...,
		),
	}

	if len(expected) == 0 {
		report.Decision = DecisionInconclusive
		report.Reason = ReasonNoExpectedEndpoints
		report.Reasons = []string{
			"control-plane resolver returned no eligible destination endpoints",
		}

		return report
	}

	forwarded := dedupeForwardedEdges(observed)

	report.ObservedEdges = forwarded

	if len(forwarded) == 0 {
		report.Decision = DecisionInconclusive
		report.Reason = ReasonNoObservedTraffic
		report.Reasons = []string{
			"no forwarded data-plane edges were observed during the measurement window",
		}

		return report
	}

	for _, edge := range forwarded {
		if !destinationExpected(
			expected,
			edge,
		) {
			report.UnexpectedEdges = append(
				report.UnexpectedEdges,
				edge,
			)
		}
	}

	if len(report.UnexpectedEdges) > 0 {
		report.Decision = DecisionFail
		report.Reason =
			ReasonControlDataPlaneDivergence

		for _, edge := range report.UnexpectedEdges {
			report.Reasons = append(
				report.Reasons,
				fmt.Sprintf(
					"unexpected forwarded edge: %s -> %s",
					sourceIdentity(edge),
					destinationIdentity(edge),
				),
			)
		}

		return report
	}

	report.Decision = DecisionPass
	report.Reasons = []string{
		"all observed forwarded destinations belong to the expected endpoint set",
	}

	return report
}

func destinationExpected(
	expected []ExpectedEndpoint,
	edge ObservedEdge,
) bool {
	haveExpectedPodIdentity := false

	for _, endpoint := range expected {
		if strings.TrimSpace(endpoint.Pod) != "" {
			haveExpectedPodIdentity = true

			if endpoint.Namespace ==
				edge.DestinationNamespace &&
				endpoint.Pod ==
					edge.DestinationPod {
				return true
			}
		}
	}

	if strings.TrimSpace(edge.DestinationPod) != "" &&
		haveExpectedPodIdentity {
		return false
	}

	if strings.TrimSpace(edge.DestinationIP) == "" {
		return false
	}

	for _, endpoint := range expected {
		if endpoint.IP != "" &&
			endpoint.IP ==
				edge.DestinationIP {
			return true
		}
	}

	return false
}

func dedupeForwardedEdges(
	edges []ObservedEdge,
) []ObservedEdge {
	unique := make(
		map[string]ObservedEdge,
	)

	for _, edge := range edges {
		if !strings.EqualFold(
			strings.TrimSpace(edge.Verdict),
			"FORWARDED",
		) {
			continue
		}

		key := edgeKey(edge)

		unique[key] = edge
	}

	keys := make(
		[]string,
		0,
		len(unique),
	)

	for key := range unique {
		keys = append(
			keys,
			key,
		)
	}

	sort.Strings(keys)

	result := make(
		[]ObservedEdge,
		0,
		len(keys),
	)

	for _, key := range keys {
		result = append(
			result,
			unique[key],
		)
	}

	return result
}

func edgeKey(
	edge ObservedEdge,
) string {
	return strings.Join(
		[]string{
			edge.SourceNamespace,
			edge.SourcePod,
			edge.SourceIP,
			edge.DestinationNamespace,
			edge.DestinationPod,
			edge.DestinationIP,
			edge.DestinationService,
		},
		"\x00",
	)
}

func sourceIdentity(
	edge ObservedEdge,
) string {
	if edge.SourcePod != "" {
		return namespaced(
			edge.SourceNamespace,
			edge.SourcePod,
		)
	}

	if edge.SourceIP != "" {
		return edge.SourceIP
	}

	return "<unknown-source>"
}

func destinationIdentity(
	edge ObservedEdge,
) string {
	if edge.DestinationPod != "" {
		return namespaced(
			edge.DestinationNamespace,
			edge.DestinationPod,
		)
	}

	if edge.DestinationIP != "" {
		return edge.DestinationIP
	}

	return "<unknown-destination>"
}

func namespaced(
	namespace string,
	name string,
) string {
	if namespace == "" {
		return name
	}

	return namespace + "/" + name
}
