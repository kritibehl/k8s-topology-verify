# Quickstart: run this against a real cluster
 
This walks through the exact environment that originally validated this tool: a local
Kind cluster running Cilium with Hubble enabled, a simple client/server workload, and
a real `k8s-topology-verify` run against it.
 
## Prerequisites
 
- Docker
- [kind](https://kind.sigs.k8s.io/)
- [cilium CLI](https://docs.cilium.io/en/stable/gettingstarted/k8s-install-default/#install-the-cilium-cli)
- kubectl
- Go 1.26+
 
## 1. Create the cluster
 
Save this as `kind-config.yaml`:
 
```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
 
networking:
  disableDefaultCNI: true
 
nodes:
  - role: control-plane
    image: kindest/node:v1.34.3@sha256:08497ee19eace7b4b5348db5c6a1591d7752b164530a36f855cb0f2bdcbadd48
```
 
`disableDefaultCNI: true` is required — Cilium replaces Kind's default CNI rather than running alongside it.
 
```bash
kind create cluster --config kind-config.yaml
```
 
## 2. Install Cilium with Hubble enabled
 
```bash
cilium install --version 1.20.0 --set hubble.relay.enabled=true --set hubble.ui.enabled=false
cilium status --wait
```
 
## 3. Deploy a client and server
 
Save this as `workloads.yaml`:
 
```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: kubepulse-hubble
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: server
  namespace: kubepulse-hubble
spec:
  replicas: 1
  selector:
    matchLabels:
      app: server
  template:
    metadata:
      labels:
        app: server
    spec:
      containers:
        - name: nginx
          image: nginx:1.27-alpine
          ports:
            - containerPort: 80
---
apiVersion: v1
kind: Service
metadata:
  name: server
  namespace: kubepulse-hubble
spec:
  selector:
    app: server
  ports:
    - port: 80
      targetPort: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: client
  namespace: kubepulse-hubble
spec:
  replicas: 1
  selector:
    matchLabels:
      app: client
  template:
    metadata:
      labels:
        app: client
    spec:
      containers:
        - name: curl
          image: curlimages/curl:8.10.1
          command:
            - sh
            - -c
            - sleep 3600
```
 
```bash
kubectl apply -f workloads.yaml
kubectl -n kubepulse-hubble wait --for=condition=available deployment/server deployment/client --timeout=120s
```
 
## 4. Generate real traffic
 
```bash
CLIENT_POD=$(kubectl -n kubepulse-hubble get pod -l app=client -o jsonpath='{.items[0].metadata.name}')
 
for i in $(seq 1 20); do
  kubectl -n kubepulse-hubble exec "$CLIENT_POD" -- curl -s -o /dev/null -w "%{http_code}\n" http://server
done
```
 
You should see 20 lines of `200`.
 
## 5. Port-forward Hubble Relay
 
In a separate terminal, leave this running:
 
```bash
cilium hubble port-forward &
```
 
This exposes Hubble Relay at `127.0.0.1:4245`, the default address `k8s-topology-verify` expects.
 
## 6. Run the verifier
 
Back in the original terminal:
 
```bash
go run ./cmd/verify \
  --namespace kubepulse-hubble \
  --service server \
  --source-selector "k8s:app=client" \
  --hubble-address 127.0.0.1:4245 \
  --window 30s
```
 
Expect `"decision": "PASS"` — the client's traffic went exactly where the `server` Service's `EndpointSlice` said it should.
 
## Try breaking it
 
To see a `FAIL`, point `--service` at a *different* service than the one the client actually talks to (e.g. deploy a second `decoy` Service selecting different pods, but keep the client hitting `server`). The tool will resolve `decoy`'s endpoints as "expected," see the client's real traffic land on `server`'s pods instead, and report `CONTROL_DATA_PLANE_DIVERGENCE`.
 
## Clean up
 
```bash
kind delete cluster
```
