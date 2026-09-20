package topology

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func boolPtr(
	value bool,
) *bool {
	return &value
}

func TestResolverUnionsEndpointSlices(
	t *testing.T,
) {
	client := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "shop",
				Name:      "payment-canary",
			},
		},

		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "shop",
				Name:      "payment-v2-a",
				Labels: map[string]string{
					"app":     "payment",
					"version": "v2",
				},
			},
		},

		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "shop",
				Name:      "payment-v2-b",
				Labels: map[string]string{
					"app":     "payment",
					"version": "v2",
				},
			},
		},

		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "shop",
				Name:      "payment-a",
				Labels: map[string]string{
					endpointSliceServiceLabel: "payment-canary",
				},
			},

			AddressType: discoveryv1.AddressTypeIPv4,

			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{
						"10.244.0.20",
					},

					Conditions: discoveryv1.EndpointConditions{
						Ready: boolPtr(true),
					},

					TargetRef: &corev1.ObjectReference{
						Kind:      "Pod",
						Namespace: "shop",
						Name:      "payment-v2-a",
					},
				},
			},
		},

		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "shop",
				Name:      "payment-b",
				Labels: map[string]string{
					endpointSliceServiceLabel: "payment-canary",
				},
			},

			AddressType: discoveryv1.AddressTypeIPv4,

			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{
						"10.244.0.21",
					},

					// nil Ready is intentionally tested
					// as eligible.
					Conditions: discoveryv1.EndpointConditions{},

					TargetRef: &corev1.ObjectReference{
						Kind:      "Pod",
						Namespace: "shop",
						Name:      "payment-v2-b",
					},
				},
			},
		},
	)

	resolver := Resolver{
		Client: client,
	}

	endpoints, err :=
		resolver.ResolveServiceEndpoints(
			context.Background(),
			"shop",
			"payment-canary",
		)

	if err != nil {
		t.Fatal(err)
	}

	if len(endpoints) != 2 {
		t.Fatalf(
			"expected 2 endpoints from two slices, got %d: %+v",
			len(endpoints),
			endpoints,
		)
	}

	if endpoints[0].Pod != "payment-v2-a" ||
		endpoints[0].IP != "10.244.0.20" {
		t.Fatalf(
			"unexpected first endpoint: %+v",
			endpoints[0],
		)
	}

	if endpoints[1].Pod != "payment-v2-b" ||
		endpoints[1].IP != "10.244.0.21" {
		t.Fatalf(
			"unexpected second endpoint: %+v",
			endpoints[1],
		)
	}

	if endpoints[0].Labels["version"] != "v2" {
		t.Fatalf(
			"expected pod labels to be preserved: %+v",
			endpoints[0].Labels,
		)
	}
}

func TestResolverExcludesNotReadyEndpoint(
	t *testing.T,
) {
	client := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "shop",
				Name:      "payment",
			},
		},

		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "shop",
				Name:      "payment",
				Labels: map[string]string{
					endpointSliceServiceLabel: "payment",
				},
			},

			AddressType: discoveryv1.AddressTypeIPv4,

			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{
						"10.244.0.30",
					},

					Conditions: discoveryv1.EndpointConditions{
						Ready: boolPtr(false),
					},
				},
			},
		},
	)

	resolver := Resolver{
		Client: client,
	}

	endpoints, err :=
		resolver.ResolveServiceEndpoints(
			context.Background(),
			"shop",
			"payment",
		)

	if err != nil {
		t.Fatal(err)
	}

	if len(endpoints) != 0 {
		t.Fatalf(
			"expected not-ready endpoint to be excluded: %+v",
			endpoints,
		)
	}
}

func TestResolverExcludesTerminatingEndpoint(
	t *testing.T,
) {
	client := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "shop",
				Name:      "payment",
			},
		},

		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "shop",
				Name:      "payment",
				Labels: map[string]string{
					endpointSliceServiceLabel: "payment",
				},
			},

			AddressType: discoveryv1.AddressTypeIPv4,

			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{
						"10.244.0.30",
					},

					Conditions: discoveryv1.EndpointConditions{
						Ready: boolPtr(true),

						Terminating: boolPtr(true),
					},
				},
			},
		},
	)

	resolver := Resolver{
		Client: client,
	}

	endpoints, err :=
		resolver.ResolveServiceEndpoints(
			context.Background(),
			"shop",
			"payment",
		)

	if err != nil {
		t.Fatal(err)
	}

	if len(endpoints) != 0 {
		t.Fatalf(
			"expected terminating endpoint to be excluded: %+v",
			endpoints,
		)
	}
}

func TestResolverSupportsIPOnlyEndpoint(
	t *testing.T,
) {
	client := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "shop",
				Name:      "external-payment",
			},
		},

		&discoveryv1.EndpointSlice{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "shop",
				Name:      "external-payment",
				Labels: map[string]string{
					endpointSliceServiceLabel: "external-payment",
				},
			},

			AddressType: discoveryv1.AddressTypeIPv4,

			Endpoints: []discoveryv1.Endpoint{
				{
					Addresses: []string{
						"10.10.10.20",
					},

					Conditions: discoveryv1.EndpointConditions{
						Ready: boolPtr(true),
					},
				},
			},
		},
	)

	resolver := Resolver{
		Client: client,
	}

	endpoints, err :=
		resolver.ResolveServiceEndpoints(
			context.Background(),
			"shop",
			"external-payment",
		)

	if err != nil {
		t.Fatal(err)
	}

	if len(endpoints) != 1 {
		t.Fatalf(
			"expected 1 IP-only endpoint, got %+v",
			endpoints,
		)
	}

	if endpoints[0].Pod != "" ||
		endpoints[0].IP != "10.10.10.20" {
		t.Fatalf(
			"unexpected IP-only endpoint: %+v",
			endpoints[0],
		)
	}
}

func TestResolverNoSlicesReturnsEmptySet(
	t *testing.T,
) {
	client := fake.NewSimpleClientset(
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "shop",
				Name:      "payment",
			},
		},
	)

	resolver := Resolver{
		Client: client,
	}

	endpoints, err :=
		resolver.ResolveServiceEndpoints(
			context.Background(),
			"shop",
			"payment",
		)

	if err != nil {
		t.Fatal(err)
	}

	if len(endpoints) != 0 {
		t.Fatalf(
			"expected empty endpoint set, got %+v",
			endpoints,
		)
	}
}

func TestResolverMissingServiceReturnsError(
	t *testing.T,
) {
	client := fake.NewSimpleClientset()

	resolver := Resolver{
		Client: client,
	}

	_, err := resolver.ResolveServiceEndpoints(
		context.Background(),
		"shop",
		"missing-service",
	)

	if err == nil {
		t.Fatal(
			"expected missing Service to return error",
		)
	}
}
