# mgtt-provider-kubernetes

Kubernetes provider for [mgtt](https://github.com/mgt-tool/mgtt) — the model guided troubleshooting tool.

Version 2.0.0 — 37 component types across workloads, networking, storage, scheduling, RBAC, webhooks, and extensibility.

## Vocabulary

The vocabulary (what the engine reasons about) lives in `provider.yaml` plus one file per type under `types/`:

| Group | Types |
|-------|-------|
| Workloads | `deployment`, `statefulset`, `daemonset`, `replicaset`, `cronjob`, `job`, `pod` |
| Networking | `service`, `ingress`, `ingressclass`, `endpoints`, `networkpolicy` |
| Scaling & availability | `hpa`, `pdb` |
| Storage | `pvc`, `persistentvolume`, `storageclass`, `csidriver`, `volumeattachment` |
| Cluster | `node`, `resourcequota`, `limitrange` |
| Prerequisites | `namespace`, `serviceaccount`, `secret`, `configmap`, `operator` |
| RBAC | `role`, `clusterrole`, `rolebinding`, `clusterrolebinding` |
| Webhooks | `validatingwebhookconfiguration`, `mutatingwebhookconfiguration` |
| Extensibility | `customresourcedefinition`, `priorityclass`, `lease`, `custom_resource` |

See `docs/design.md` for the rationale and per-type state machines.

## Runtime status

The runner binary (`main.go`) currently implements probes for `deployment` and `ingress` only. The other 35 types are declared in the vocabulary so the engine can reason about them, but probing them at runtime will fail until the runner is expanded — tracked as follow-up work.

## Install

```bash
mgtt provider install kubernetes
```

The install hook compiles the provider binary from source (requires Go).

## Auth

Uses your existing kubeconfig: `KUBECONFIG`, `~/.kube/config`, or in-cluster service account. Read-only kubectl access only.
