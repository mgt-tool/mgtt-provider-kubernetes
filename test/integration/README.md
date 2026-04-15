# Integration tests

End-to-end tests that prove the provider works against a real Kubernetes cluster and is usable by `mgtt` running in docker.

## What they cover

1. **`TestProbeRunner_AgainstRealCluster`** — builds the provider binary, spins up a `kind` cluster, deploys an nginx Deployment, invokes the binary directly, and asserts the JSON it returns matches the cluster's actual `readyReplicas` / `desiredReplicas` / `restartCount`.

2. **`TestMgttDocker_ProviderInspect`** — stages the provider under a temp `$MGTT_HOME/providers/kubernetes/`, runs the `mgtt` docker image with that home bind-mounted, executes `mgtt provider inspect kubernetes`, and asserts the v2.0.0 vocabulary (37 types) is discovered. This proves `mgtt` can load a multi-file provider end-to-end.

3. **`TestMgttDocker_ModelValidate`** — validates a sample model that declares a `type: deployment` component, via `mgtt model validate` inside the docker image. Passing validation means `mgtt` loaded the kubernetes provider and resolved the type.

## Two ways to run

### A. Native (fastest, requires host tooling)

Requirements: `docker`, `kind`, `kubectl`, `go` on `$PATH`. Network access to pull `ghcr.io/mgt-tool/mgtt:latest` (or override with `MGTT_IMAGE`).

```bash
go test -tags=integration ./test/integration/...
```

The test suite creates a kind cluster named `mgtt-provider-kubernetes-it` and reuses it across runs. Delete it manually when you're done:

```bash
kind delete cluster --name mgtt-provider-kubernetes-it
```

### B. Hermetic (docker-in-docker, zero host install)

Requirements: `docker` only. Everything else (`go`, `kubectl`, `kind`, the inner Docker daemon, the kind cluster, pulled images, Go build/module cache) lives inside a disposable container + named volumes. Nothing lands on the host filesystem or the host Docker daemon.

```bash
cd test/integration
docker compose run --rm tester
```

Pass extra `go test` args after the service name:

```bash
docker compose run --rm tester go test -tags=integration -v -run TestProbeRunner ./test/integration/...
```

`MGTT_IMAGE` defaults to `ghcr.io/mgt-tool/mgtt:latest` in `docker-compose.yml`, so the `TestMgttDocker_*` tests run out of the box **if the inner daemon can pull that image**. The `mgt-tool/mgtt` package on ghcr.io is currently published as private; without credentials, the entrypoint clears `MGTT_IMAGE` after a failed prewarm and those two tests skip cleanly. Override with a public tag, or build from source:

```bash
# Pin a specific tag (still subject to ghcr.io auth)
MGTT_IMAGE=ghcr.io/mgt-tool/mgtt:v1.2.3 docker compose run --rm tester

# Build the mgtt image from a local source checkout (no registry auth needed)
MGTT_IMAGE= MGTT_SRC=/path/to/mgtt docker compose run --rm tester
```

Full cleanup — removes the tester image, the inner daemon's entire state, the kind cluster (which lived inside the inner daemon), and all Go caches:

```bash
docker compose down -v --rmi local
```

**What persists across runs.** The inner Docker daemon's state lives on the `tester-docker` named volume — pulled images (kindest/node, nginx, mgtt if reachable) survive between invocations. Go's build and module caches sit on `tester-gocache` and `tester-gomod`. The kind **cluster itself** is deleted and recreated on every run for clean test state (the alternative — reusing the cluster — meant pod restarts from previous tester exits bumped `restart_count` above 0 and tripped the probe assertion). Cluster recreate against the warm image cache costs ~17s.

**Tradeoffs vs. native:** slower first run (inner daemon pulls everything from scratch, ~90s end-to-end), requires `privileged: true`. Subsequent runs are ~30–35s.

