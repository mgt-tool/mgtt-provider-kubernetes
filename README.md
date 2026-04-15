# mgtt-provider-kubernetes

Kubernetes provider for [mgtt](https://github.com/mgt-tool/mgtt) — the model guided troubleshooting tool.

Version **2.1.0** — built on the [mgtt provider SDK](https://github.com/mgt-tool/mgtt/tree/main/sdk/provider) (requires mgtt ≥ 0.1.0).

## Vocabulary

The vocabulary (what the engine reasons about) lives in `provider.yaml` plus one file per type under `types/`. All 37 types are declared so the engine can reason about them; runtime probes are implemented in tiers:

| Group | Types | Runtime tier |
|---|---|---|
| Workloads | `deployment`, `statefulset`, `daemonset`, `pod` | ✅ Tier 1 |
| Workloads (deferred) | `replicaset`, `cronjob`, `job` | ⏳ Tier 2 |
| Networking | `service`, `endpoints`, `ingress` | ✅ Tier 1 |
| Networking (deferred) | `networkpolicy`, `ingressclass` | ⏳ Tier 2 |
| Scaling | `hpa` | ✅ Tier 1 |
| Scaling (deferred) | `pdb` | ⏳ Tier 2 |
| Storage | `pvc` | ✅ Tier 1 |
| Storage (deferred) | `persistentvolume`, `storageclass`, `csidriver`, `volumeattachment` | ⏳ Tier 2/3 |
| Cluster | `node` | ✅ Tier 1 |
| Cluster (deferred) | `resourcequota`, `limitrange` | ⏳ Tier 2 |
| Prereqs, RBAC, Webhooks, Extensibility | all | ⏳ Tier 2/3 |

Tier 1 covers the workload-path types operators most often troubleshoot. Tier 2/3 vocabulary is still visible to the engine for reasoning; invoking a probe on a Tier 2/3 type will surface `usage error: unknown fact`. The rationale lives in `docs/superpowers/plans/2026-04-15-provider-hardening.md` (S4 triage).

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
