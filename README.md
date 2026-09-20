# k8s-topology-verify

**Detects when Kubernetes reports a workload as healthy while the real Cilium/Hubble data plane disagrees.**

A canary can show `1/1 Ready` while the actual network traffic it receives — observed at the packet level via eBPF — is silently degrading or reaching a backend nobody intended. Readiness probes and control-plane state can't see that; this tool can, because it checks both independently and diffs them.

`Go` · `Kubernetes client-go` · `Cilium Hubble`

---

## What it does

1. **Resolves control-plane intent** — reads the target `Service` and its `EndpointSlices`, excluding not-ready and terminating backends, to build the expected destination set.
2. **Observes the real data plane** — streams live flow data from Hubble Relay for the workload being checked, keeping only `FORWARDED` edges (a drop doesn't prove an unexpected destination was reached).
3. **Compares the two** — pod identity is preferred when available; IP is used only as a fallback when Hubble didn't resolve a pod name. Emits an explicit decision:

```json
{
  "decision": "FAIL",
  "reason": "CONTROL_DATA_PLANE_DIVERGENCE",
  "reasons": ["unexpected forwarded edge: default/canary-abc123 -> default/payment-v1-xyz"]
}
```

`PASS`, `FAIL`, or `INCONCLUSIVE` (with an explicit reason — e.g. no expected endpoints resolved, or no forwarded traffic observed in the window) rather than a false confident answer when evidence is thin.

## Why this exists

Originally built as a metric-provider plugin for Argo Rollouts, where it caught a real regression: a canary reporting `1/1 Ready` throughout, while Hubble showed its flow-event drop rate rise from 0% to 43.4% (2,219 stable / 106 canary flow events observed). The rollout was auto-aborted while the healthy stable ReplicaSet stayed available.

This repo pulls the detection engine out of that Argo-specific wrapper so it's usable standalone — no Argo Rollouts required, just a Kubernetes cluster running Cilium with Hubble enabled.

## Use it

```bash
go run ./cmd/verify \
  --namespace default \
  --service payment-v2 \
  --source-selector "k8s:app=payment,role=canary" \
  --hubble-address 127.0.0.1:4245 \
  --window 30s
```

Exit codes: `0` = PASS, `1` = FAIL, `3` = INCONCLUSIVE — distinct from FAIL, since "not enough evidence" and "confirmed divergence" call for different responses in a CI/CD gate.

## Also usable as a library

```go
import (
    "github.com/kritibehl/k8s-topology-verify/topology"
    "github.com/kritibehl/k8s-topology-verify/hubble"
)

resolver := &topology.Resolver{Client: clientset}
expected, _ := resolver.ResolveServiceEndpoints(ctx, "default", "payment-v2")

hubbleClient, _ := hubble.Dial(ctx, "127.0.0.1:4245")
collection, _ := hubbleClient.CollectObservedEdges(ctx, "k8s:app=payment,role=canary", since, until)

report := topology.Compare(expected, collection.Edges)
```

This is how it's wired into an Argo Rollouts `AnalysisRun` in [`integrations/argo-rollouts/`](integrations/argo-rollouts/) — a working reference if you want to use this with Argo specifically, but the core `topology`/`hubble` packages have no Argo dependency at all.

## Tests

```bash
go test ./topology/... ./hubble/... -v
```

18 tests covering: expected/observed endpoint matching, partial observation of an expected backend set, unexpected-destination detection, pod-identity precedence over IP fallback, dropped-edge handling (a drop doesn't prove reachability), missing-endpoint and missing-traffic inconclusive cases, repeated-flow deduplication, EndpointSlice union across multiple slices, not-ready/terminating exclusion, and IP-only endpoint support. The Hubble client is tested against an in-process gRPC server, not mocked at the interface level.

## Scope

This verifies Layer 3/4 reachability and pod-identity consistency between control-plane intent and Hubble-observed data-plane behavior. It does not claim to verify application-layer correctness, packet-loss percentages (these are Hubble flow-event rates, not deduplicated request counts), or behavior outside the observation window.
