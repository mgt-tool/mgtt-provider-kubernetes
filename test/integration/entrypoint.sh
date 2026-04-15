#!/bin/sh
set -e

# Start the inner Docker daemon in the background.
dockerd-entrypoint.sh dockerd \
    --host=unix:///var/run/docker.sock \
    --storage-driver=overlay2 \
    >/var/log/dockerd.log 2>&1 &

# Wait for it to become ready.
for i in $(seq 1 30); do
    if docker info >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

if ! docker info >/dev/null 2>&1; then
    echo "inner dockerd failed to start; tail of /var/log/dockerd.log:" >&2
    tail -n 50 /var/log/dockerd.log >&2 || true
    exit 1
fi

# Always start with a fresh kind cluster. The previous tester's dockerd died
# with the container, restarting the kind node and bumping pod restart_count —
# which trips the test's "0 restarts on a healthy workload" assertion. The
# kindest/node and nginx images stay cached on the tester-docker volume, so
# cluster recreate is fast (~17s on a warm cache) and bulletproof.
existing=$(kind get clusters 2>/dev/null | head -n1)
if [ -n "$existing" ]; then
    echo "deleting stale kind cluster '$existing' for clean test state..." >&2
    kind delete cluster --name "$existing" >/dev/null 2>&1 || true
fi

# Prewarm the mgtt image into the inner daemon's cache (persisted on the
# tester-docker volume, so this cost is paid once per volume lifetime).
# If the pull fails (e.g. ghcr.io needs auth) and the image isn't already
# cached, unset MGTT_IMAGE so TestMgttDocker_* skip cleanly via the test's
# own fallback (MGTT_SRC, then ../mgtt sibling, then t.Skip).
if [ -n "${MGTT_IMAGE}" ]; then
    if ! docker image inspect "${MGTT_IMAGE}" >/dev/null 2>&1; then
        echo "prewarming ${MGTT_IMAGE}..." >&2
        if ! docker pull "${MGTT_IMAGE}"; then
            echo "warning: failed to pull ${MGTT_IMAGE}; clearing MGTT_IMAGE so tests skip cleanly (set MGTT_SRC to build from source instead)" >&2
            export MGTT_IMAGE=""
        fi
    fi
fi

exec "$@"
