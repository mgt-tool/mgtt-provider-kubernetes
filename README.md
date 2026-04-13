# mgtt-provider-kubernetes

Kubernetes provider for [mgtt](https://github.com/sajonaro/mgtt) — the model guided troubleshooting tool.

## Types

| Type | Description | Facts |
|------|-------------|-------|
| `ingress` | Kubernetes ingress / reverse proxy | `upstream_count` (int) |
| `deployment` | Kubernetes Deployment | `ready_replicas` (int), `desired_replicas` (int), `restart_count` (int), `endpoints` (int) |

## Install

```bash
mgtt provider install kubernetes
```

The install hook compiles the provider binary from source (requires Go).

## Auth

Uses your existing kubeconfig: `KUBECONFIG`, `~/.kube/config`, or in-cluster service account. Read-only kubectl access only.
