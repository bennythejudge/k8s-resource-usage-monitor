# An experiment with Kubernetes API aggregation layer

⚠️ **DISCLAIMER: This is an Experimental Project**

This project is provided **as-is** for educational and experimental purposes only. It is **NOT** intended for use in any production or non-production Kubernetes cluster. The author takes **NO RESPONSIBILITY** for any damage, data loss, security breaches, or misuse of this software.

See [LICENSE](LICENSE) for full terms and disclaimer.

---

- A tiny Kubernetes API extension to get a JSON showing all Pods with their `resources` and actual CPU/Mem usage via `metric-server`

## Prerequisites

- a `minikube` cluster

- `kubectl`, `docker`, `go` (for local build), `task`

- the `metrics-server`

## Steps

### Install `minikube`

```bash
task start
```

### Install the `metrics-server`

```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

or

```bash
minikube start --addons metrics-server
```

or

```bash
task start
```

### TLS

- API aggregation requires HTTPS. We need to generate a self-signed CA and server certificate

Check in [`Taskfile.yaml`](Taskfile.yaml) to find the actual `openssl` commands or use

```bash
task certs
```

### Build image with that exact tag

```bash
TAG=0.0.9 task build
```

### Load image into Minikube

```bash
minikube image load k8s-efficiency-auditor:0.0.9
```

or

```bash
TAG=0.0.9 task load
```

### Install/upgrade using Helm

```bash
TAG=0.0.9 task deploy
```

### Watch the pod come up

```bash
kubectl get pods -n kube-system -w
```

### Check API

```bash
kubectl api-resource | grep pod-resource

kubectl get --raw "/apis/example.com/v1/namespaces/kube-system/pod-resources"
```

The `Taskfile` contains other steps - check with `task -l`
