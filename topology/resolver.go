package topology

import (
	"context"
	"fmt"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const endpointSliceServiceLabel = "kubernetes.io/service-name"

type Resolver struct {
	Client kubernetes.Interface
}

func (r *Resolver) ResolveServiceEndpoints(
	ctx context.Context,
	namespace string,
	serviceName string,
) ([]ExpectedEndpoint, error) {
	if r == nil || r.Client == nil {
		return nil, fmt.Errorf(
			"kubernetes client is not initialized",
		)
	}

	namespace = strings.TrimSpace(namespace)
	serviceName = strings.TrimSpace(serviceName)

	if namespace == "" {
		return nil, fmt.Errorf(
			"namespace must not be empty",
		)
	}

	if serviceName == "" {
		return nil, fmt.Errorf(
			"service name must not be empty",
		)
	}

	// Verify the Service itself exists.
	if _, err := r.Client.
		CoreV1().
		Services(namespace).
		Get(
			ctx,
			serviceName,
			metav1.GetOptions{},
		); err != nil {

		return nil, fmt.Errorf(
			"get service %s/%s: %w",
			namespace,
			serviceName,
			err,
		)
	}

	slices, err := r.Client.
		DiscoveryV1().
		EndpointSlices(namespace).
		List(
			ctx,
			metav1.ListOptions{
				LabelSelector: endpointSliceServiceLabel +
					"=" +
					serviceName,
			},
		)

	if err != nil {
		return nil, fmt.Errorf(
			"list EndpointSlices for service %s/%s: %w",
			namespace,
			serviceName,
			err,
		)
	}

	unique := make(
		map[string]ExpectedEndpoint,
	)

	for _, slice := range slices.Items {
		for _, endpoint := range slice.Endpoints {
			if !eligibleEndpoint(endpoint.Conditions.Ready,
				endpoint.Conditions.Terminating) {
				continue
			}

			podNamespace := namespace
			podName := ""

			if endpoint.TargetRef != nil &&
				endpoint.TargetRef.Kind == "Pod" &&
				endpoint.TargetRef.Name != "" {

				podName =
					endpoint.TargetRef.Name

				if endpoint.TargetRef.Namespace != "" {
					podNamespace =
						endpoint.TargetRef.Namespace
				}
			}

			var podLabels map[string]string

			if podName != "" {
				pod, err := r.Client.
					CoreV1().
					Pods(podNamespace).
					Get(
						ctx,
						podName,
						metav1.GetOptions{},
					)

				if err != nil {
					return nil, fmt.Errorf(
						"get EndpointSlice target pod %s/%s: %w",
						podNamespace,
						podName,
						err,
					)
				}

				podLabels =
					copyStringMap(pod.Labels)
			}

			for _, address := range endpoint.Addresses {
				address = strings.TrimSpace(
					address,
				)

				if address == "" {
					continue
				}

				expected := ExpectedEndpoint{
					Namespace: podNamespace,
					Pod:       podName,
					IP:        address,
					Labels:    podLabels,
				}

				unique[expectedEndpointKey(expected)] =
					expected
			}
		}
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
		[]ExpectedEndpoint,
		0,
		len(keys),
	)

	for _, key := range keys {
		result = append(
			result,
			unique[key],
		)
	}

	return result, nil
}

func eligibleEndpoint(
	ready *bool,
	terminating *bool,
) bool {
	// Kubernetes EndpointSlice semantics:
	// nil ready is treated as eligible here.
	if ready != nil && !*ready {
		return false
	}

	// A terminating backend should not be considered
	// part of the current expected forwarding set.
	if terminating != nil && *terminating {
		return false
	}

	return true
}

func expectedEndpointKey(
	endpoint ExpectedEndpoint,
) string {
	return strings.Join(
		[]string{
			endpoint.Namespace,
			endpoint.Pod,
			endpoint.IP,
		},
		"\x00",
	)
}

func copyStringMap(
	input map[string]string,
) map[string]string {
	if len(input) == 0 {
		return nil
	}

	output := make(
		map[string]string,
		len(input),
	)

	for key, value := range input {
		output[key] = value
	}

	return output
}
