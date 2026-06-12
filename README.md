⚠️ **DISCLAIMER: This is an Experimental Project**

This project is provided **as-is** for educational and experimental purposes only. It is **NOT** intended for use in any production or non-production Kubernetes cluster. The author takes **NO RESPONSIBILITY** for any damage, data loss, security breaches, or misuse of this software.

See [LICENSE](LICENSE) for full terms and disclaimer.

---

# An experiment with Kubernetes API aggregation layer

- A tiny Kubernetes API extension to get a JSON showing all Pods with their `resources` and actual CPU/Mem usage via `metric-server`

## Prerequisites

- a `minikube` cluster

- `kubectl`, `docker`, `go` (for local build), `task`

- the `metrics-server`

## Steps

### Start `minikube` and the `metrics-server`

```bash
task start
```

### Create TLS

- API aggregation requires HTTPS. We need to generate a self-signed CA and server certificate

```bash
task certs
```

### Build image with that exact tag

```bash
TAG=0.0.9 task build
```

### Load image into Minikube

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
