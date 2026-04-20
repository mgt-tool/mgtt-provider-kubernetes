# mgtt-provider-kubernetes

Kubernetes provider for [mgtt](https://github.com/mgt-tool/mgtt) — the model guided troubleshooting tool.

Version **3.0.0** — built on the [mgtt provider SDK](https://github.com/mgt-tool/mgtt/tree/main/sdk/provider) (requires mgtt ≥ 0.2.0).

## Vocabulary

The vocabulary (what the engine reasons about) lives in `manifest.yaml` plus one file per type under `types/`. All 37 types have runtime probes.

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

Two equivalent paths — pick whichever fits your workflow:

```bash
# Git + host toolchain (requires Go 1.25+, warns if kubectl not on PATH)
mgtt provider install kubernetes

# Pre-built Docker image (ships kubectl inside; digest-pinned)
mgtt provider install --image ghcr.io/mgt-tool/mgtt-provider-kubernetes:3.0.0@sha256:...
```

The image is published by [this repo's CI](./.github/workflows/docker.yml) on every push to `main` and every `v*` tag. Find the current digest on the [GHCR package page](https://github.com/mgt-tool/mgtt-provider-kubernetes/pkgs/container/mgtt-provider-kubernetes).

## Capabilities

When installed as an image, this provider declares the following runtime capabilities in [`manifest.yaml`](./manifest.yaml) (top-level `needs:`):

| Capability | Effect at probe time |
|---|---|
| `kubectl` | Mounts `~/.kube` read-only; forwards `KUBECONFIG` so in-container `kubectl` picks up the same context the operator's CLI uses |

Plus `network: host` so the container reaches the cluster API server (in-cluster URLs, private hostnames, service CIDR DNS).

For a non-default kubeconfig path, override the `kubectl` capability in `$MGTT_HOME/capabilities.yaml`. See the [capability reference](https://github.com/mgt-tool/mgtt/blob/main/docs/reference/image-capabilities.md).

## Auth

Uses your existing kubeconfig: `KUBECONFIG`, `~/.kube/config`, or in-cluster service account. The provider is **read-only** — `manifest.yaml` omits `read_only:` (defaults to `true`), and every probe is a `kubectl get`/`list`/`watch`. Operators should bind a ServiceAccount whose ClusterRole matches, with no `create`/`update`/`delete`/`patch`/`deletecollection` verbs on the probed resources.

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

Integration tests also have a hermetic docker-in-docker runner that needs only `docker` on the host (no `kind`/`kubectl`/`go`):

```bash
cd test/integration && docker compose run --rm tester
```

See [`test/integration/README.md`](test/integration/README.md) for details.

## License

[Apache License 2.0](LICENSE). Matches the license of the [mgtt provider SDK](https://github.com/mgt-tool/mgtt/tree/main/sdk/provider) this provider links against; pick-up by downstream packagers and distributors should be friction-free.

Note that the [mgtt core engine](https://github.com/mgt-tool/mgtt) is licensed separately under AGPL-3.0 — that copyleft applies to the engine itself, not to providers that consume the SDK.
