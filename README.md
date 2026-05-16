# Numa-Aware Kubernetes Scheduler
A custom Kubernetes scheduler built with the Scheduler Framework to place resource-intensive Pods intelligently based on underlying server NUMA (Non-Uniform Memory Access) microarchitecture.

# Technologies
[![Go](https://img.shields.io/badge/-Go-464646?style=flat-square&logo=go)](https://go.dev/)
[![Kubernetes](https://img.shields.io/badge/-Kubernetes-464646?style=flat-square&logo=kubernetes)](https://kubernetes.io/)
[![Docker](https://img.shields.io/badge/-Docker-464646?style=flat-square&logo=docker)](https://www.docker.com/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Tech Stack

### Languages & Frameworks
- **Programming Language**: Go 1.22
- **Kubernetes Scheduler Framework**: k8s.io/kubernetes v1.30.0

### Kubernetes Concepts
- **Custom Resource Definitions (CRDs)**: `NumaTopology` CRD for defining and persisting hardware state natively within the cluster.
- **DaemonSet**: Background agent discovering hardware node layout.
- **RBAC**: Strictly scoped roles and bindings for security.
- **Dynamic Client**: Used by the agent for flexible CRD interactions.

## Architecture

The project consists of two interacting components ensuring compute-heavy Pods hit their optimal NUMA boundary, significantly reducing memory access latency and improving L3 cache hit rates.

### 1. Hardware Discovery Agent (DaemonSet)
A lightweight Go daemon running on every node.
- Reads hardware layout (NUMA zones, CPU arrays, memory banks) via `/sys/devices/system/node`.
- Authenticates securely via Kubernetes In-Cluster config.
- Pushes topological maps into a Kubernetes Custom Resource (`NumaTopology`).

### 2. Custom Scheduler Plugin
A Kubernetes Scheduler compiled with our custom `NumaAwarePlacement` logic.
- **Filter**: Rejects candidate nodes if a Pod requests more CPUs than any *single* NUMA zone on that server can provide.
- **Score**: Implements a bin-packing heuristic; the tighter the pod fits into a NUMA zone leaving the least fragmented CPUs behind, the higher the node scores.

## Project Structure

```text
numa-aware-scheduler/
├── cmd/
│   ├── scheduler/         # Entry point for the custom kube-scheduler
│   │   └── main.go
│   └── agent/             # Entry point for the hardware discovery DaemonSet
│       └── main.go
├── configs/               # Scheduler configuration files
│   └── scheduler-config.yaml
├── k8s/                   # Kubernetes deployment manifests
│   ├── crd/
│   │   └── numatopologies.yaml
│   ├── agent/
│   │   ├── daemonset.yaml
│   │   └── rbac.yaml
│   └── scheduler/
│       ├── deployment.yaml
│       └── rbac.yaml
├── internal/
│   ├── agent/             # DaemonSet Agent specific logic
│   │   ├── discovery/     # Hardware topology extraction (sysfs)
│   │   └── k8sclient/     # Dynamic K8s API communication
│   └── scheduler/         # Scheduler specific logic
│       └── plugins/
│           └── numaware/  # Filter and Score logic implementation
├── pkg/
│   └── api/               # Shared API definitions
│       └── v1/            # CRD Go types and deepcopy definitions
├── Dockerfile.scheduler   # Container build for the scheduler
├── Dockerfile.agent       # Container build for the agent
├── fix-gomod.sh           # Bash script fixing k8s staging go.mod replacements
├── go.mod
└── go.sum
```

## Quick Start

### Requirements
- Go 1.22+
- Docker
- A running Kubernetes cluster (v1.30.0+ recommended) with `admin` context configured.

### Build and Deploy

1. **Build Docker Images** (Assuming a local registry like Kind/Minikube or a remote you have access to):
```bash
docker build -t k8s.gcr.io/numa-aware-scheduler:latest -f Dockerfile.scheduler .
docker build -t k8s.gcr.io/numa-discovery-agent:latest -f Dockerfile.agent .
```
*(Note: If pushing to a remote registry, update the image URLs in `k8s/agent/daemonset.yaml` and `k8s/scheduler/deployment.yaml`)*

2. **Deploy to Cluster**:
```bash
# 1. Apply the CRD
kubectl apply -f k8s/crd/numatopologies.yaml

# 2. Deploy the Hardware Discovery Agent
kubectl apply -f k8s/agent/rbac.yaml
kubectl apply -f k8s/agent/daemonset.yaml

# 3. Deploy the Custom Scheduler
kubectl apply -f k8s/scheduler/rbac.yaml
kubectl apply -f k8s/scheduler/deployment.yaml
```

3. **Verify Deployment**:
```bash
# Check if agent correctly identified hardware topologies
kubectl get numatopologies

# Check logs of the scheduler
kubectl logs -n kube-system -l component=scheduler
```

### Usage
To instruct Kubernetes to use your new custom scheduler for a specific workload, define the `schedulerName` inside your Pod's spec:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: compute-heavy-workload
spec:
  schedulerName: numa-aware-scheduler
  containers:
  - name: workload
    image: my-compute-image:latest
    resources:
      requests:
        cpu: "8" # The Numa-Aware Scheduler will evaluate NUMA zones to fulfill this request
```

## Testing & CI

This repository uses GitHub Actions for Continuous Integration.
The CI pipeline automatically runs `go test -coverprofile` and rigorous linting via `golangci-lint` on every push and PR to the `main` branch.

To run tests locally:
```bash
go test -v -cover ./...
```
