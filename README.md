# mgtt-provider-kubernetes

Kubernetes provider for [mgtt](https://github.com/mgt-tool/mgtt) — the model guided troubleshooting tool.

Version **2.3.0** — built on the [mgtt provider SDK](https://github.com/mgt-tool/mgtt/tree/main/sdk/provider) (requires mgtt ≥ 0.1.0).

## Vocabulary

The vocabulary (what the engine reasons about) lives in `provider.yaml` plus one file per type under `types/`. All 37 types have runtime probes.

| Group | Types |
|---|---|
| Workloads | `deployment`, `statefulset`, `daemonset`, `replicaset`, `pod`, `job`, `cronjob` |
| Networking | `service`, `endpoints`, `ingress`, `ingressclass`, `networkpolicy` |
| Scaling & availability | `hpa`, `pdb` |
| Storage | `pvc`, `persistentvolume`, `storageclass`, `csidriver`, `volumeattachment` |
| Cluster | `node`, `resourcequota`, `limitrange` |
| Prerequisites | `namespace`, `serviceaccount`, `secret`, `configmap`, `operator` |
| RBAC | `role`, `clusterrole`, `rolebinding`, `clusterrolebinding` |
| Webhooks | `validatingwebhookconfiguration`, `mutatingwebhookconfiguration` |
| Extensibility | `customresourcedefinition`, `custom_resource`, `priorityclass`, `lease` |

See `docs/design.md` for the per-type state machines.

## Install

```bash
mgtt provider install kubernetes
```

The install hook gates on Go 1.21+ and warns if `kubectl` is not yet on PATH at install time.

## Auth

Uses your existing kubeconfig: `KUBECONFIG`, `~/.kube/config`, or in-cluster service account. The provider is **read-only** — `auth.access.writes: none` in `provider.yaml`. Operators should bind a ServiceAccount whose ClusterRole has only `get`/`list`/`watch` verbs on the probed resources.

## Architecture

The provider is a thin wiring layer on top of the mgtt SDK:

- `main.go` — 13 lines: registers types and calls `provider.Main`.
- `internal/probes/` — one file per type. Each registers ProbeFns against a `provider.Registry`.
- `internal/kubeclassify/` — the **only** place kubectl stderr phrasing (`NotFound`, `Forbidden`, `Unable to connect`) is recognized; maps those to the SDK's sentinel errors.

Plumbing (argv parsing, timeouts, size caps, `status:not_found` translation, exit codes, `version` subcommand, debug tracing) comes from the SDK.

## Development

```bash
go build .                    # compile locally
go test -race ./...           # unit tests
go test -tags=integration ./test/integration/...   # integration (needs kind)
mgtt provider validate kubernetes                  # static correctness
```
