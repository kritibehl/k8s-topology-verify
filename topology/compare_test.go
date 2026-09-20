package topology

import "testing"

func expectedPayments() []ExpectedEndpoint {
	return []ExpectedEndpoint{
		{
			Namespace: "shop",
			Pod:       "payment-v2-a",
			IP:        "10.244.0.20",
			Labels: map[string]string{
				"app":     "payment",
				"version": "v2",
			},
		},
		{
			Namespace: "shop",
			Pod:       "payment-v2-b",
			IP:        "10.244.0.21",
			Labels: map[string]string{
				"app":     "payment",
				"version": "v2",
			},
		},
	}
}

func TestExpectedObservedEndpointPasses(
	t *testing.T,
) {
	report := Compare(
		expectedPayments(),
		[]ObservedEdge{
			{
				SourceNamespace:      "shop",
				SourcePod:            "checkout-v2",
				SourceIP:             "10.244.0.10",
				DestinationNamespace: "shop",
				DestinationPod:       "payment-v2-a",
				DestinationIP:        "10.244.0.20",
				DestinationService:   "payment-canary",
				Verdict:              "FORWARDED",
			},
		},
	)

	if report.Decision != DecisionPass {
		t.Fatalf(
			"expected PASS, got %s: %+v",
			report.Decision,
			report,
		)
	}
}

func TestNotEveryExpectedEndpointMustBeObserved(
	t *testing.T,
) {
	report := Compare(
		expectedPayments(),
		[]ObservedEdge{
			{
				SourceNamespace:      "shop",
				SourcePod:            "checkout-v2",
				DestinationNamespace: "shop",
				DestinationPod:       "payment-v2-b",
				DestinationIP:        "10.244.0.21",
				DestinationService:   "payment-canary",
				Verdict:              "FORWARDED",
			},
		},
	)

	if report.Decision != DecisionPass {
		t.Fatalf(
			"expected PASS when one valid load-balanced endpoint is observed, got %s: %+v",
			report.Decision,
			report,
		)
	}
}

func TestUnexpectedDestinationFails(
	t *testing.T,
) {
	report := Compare(
		expectedPayments(),
		[]ObservedEdge{
			{
				SourceNamespace:      "shop",
				SourcePod:            "checkout-v2",
				DestinationNamespace: "shop",
				DestinationPod:       "payment-v1-c",
				DestinationIP:        "10.244.0.30",
				DestinationService:   "payment-canary",
				Verdict:              "FORWARDED",
			},
		},
	)

	if report.Decision != DecisionFail {
		t.Fatalf(
			"expected FAIL, got %s: %+v",
			report.Decision,
			report,
		)
	}

	if report.Reason !=
		ReasonControlDataPlaneDivergence {
		t.Fatalf(
			"expected divergence reason, got %q",
			report.Reason,
		)
	}

	if len(report.UnexpectedEdges) != 1 {
		t.Fatalf(
			"expected 1 unexpected edge, got %d",
			len(report.UnexpectedEdges),
		)
	}
}

func TestPodIdentityWinsOverIPFallback(
	t *testing.T,
) {
	expected := []ExpectedEndpoint{
		{
			Namespace: "shop",
			Pod:       "payment-v2",
			IP:        "10.244.0.20",
		},
	}

	observed := []ObservedEdge{
		{
			SourceNamespace:      "shop",
			SourcePod:            "checkout-v2",
			DestinationNamespace: "shop",
			DestinationPod:       "payment-v1",
			DestinationIP:        "10.244.0.20",
			Verdict:              "FORWARDED",
		},
	}

	report := Compare(
		expected,
		observed,
	)

	if report.Decision != DecisionFail {
		t.Fatalf(
			"expected FAIL for wrong pod identity even with matching IP, got %s: %+v",
			report.Decision,
			report,
		)
	}
}

func TestIPFallbackWhenHubblePodIdentityMissing(
	t *testing.T,
) {
	report := Compare(
		expectedPayments(),
		[]ObservedEdge{
			{
				SourceNamespace:    "shop",
				SourcePod:          "checkout-v2",
				DestinationIP:      "10.244.0.20",
				DestinationService: "payment-canary",
				Verdict:            "FORWARDED",
			},
		},
	)

	if report.Decision != DecisionPass {
		t.Fatalf(
			"expected PASS from destination IP fallback, got %s: %+v",
			report.Decision,
			report,
		)
	}
}

func TestDroppedUnexpectedEdgeDoesNotProveReachability(
	t *testing.T,
) {
	report := Compare(
		expectedPayments(),
		[]ObservedEdge{
			{
				SourceNamespace:      "shop",
				SourcePod:            "checkout-v2",
				DestinationNamespace: "shop",
				DestinationPod:       "payment-v1",
				DestinationIP:        "10.244.0.30",
				Verdict:              "DROPPED",
			},
			{
				SourceNamespace:      "shop",
				SourcePod:            "checkout-v2",
				DestinationNamespace: "shop",
				DestinationPod:       "payment-v2-a",
				DestinationIP:        "10.244.0.20",
				Verdict:              "FORWARDED",
			},
		},
	)

	if report.Decision != DecisionPass {
		t.Fatalf(
			"expected PASS because only forwarded destinations define realized topology, got %s: %+v",
			report.Decision,
			report,
		)
	}
}

func TestNoExpectedEndpointsIsInconclusive(
	t *testing.T,
) {
	report := Compare(
		nil,
		[]ObservedEdge{
			{
				DestinationPod: "payment-v2",
				Verdict:        "FORWARDED",
			},
		},
	)

	if report.Decision !=
		DecisionInconclusive {
		t.Fatalf(
			"expected INCONCLUSIVE, got %s",
			report.Decision,
		)
	}

	if report.Reason !=
		ReasonNoExpectedEndpoints {
		t.Fatalf(
			"expected %q, got %q",
			ReasonNoExpectedEndpoints,
			report.Reason,
		)
	}
}

func TestNoForwardedTrafficIsInconclusive(
	t *testing.T,
) {
	report := Compare(
		expectedPayments(),
		[]ObservedEdge{
			{
				DestinationPod: "payment-v1",
				Verdict:        "DROPPED",
			},
		},
	)

	if report.Decision !=
		DecisionInconclusive {
		t.Fatalf(
			"expected INCONCLUSIVE, got %s",
			report.Decision,
		)
	}

	if report.Reason !=
		ReasonNoObservedTraffic {
		t.Fatalf(
			"expected %q, got %q",
			ReasonNoObservedTraffic,
			report.Reason,
		)
	}
}

func TestRepeatedFlowEventsAreDeduplicated(
	t *testing.T,
) {
	edge := ObservedEdge{
		SourceNamespace:      "shop",
		SourcePod:            "checkout-v2",
		DestinationNamespace: "shop",
		DestinationPod:       "payment-v2-a",
		DestinationIP:        "10.244.0.20",
		DestinationService:   "payment-canary",
		Verdict:              "FORWARDED",
	}

	report := Compare(
		expectedPayments(),
		[]ObservedEdge{
			edge,
			edge,
			edge,
		},
	)

	if report.Decision != DecisionPass {
		t.Fatalf(
			"expected PASS, got %s",
			report.Decision,
		)
	}

	if len(report.ObservedEdges) != 1 {
		t.Fatalf(
			"expected 1 deduplicated edge, got %d",
			len(report.ObservedEdges),
		)
	}
}
