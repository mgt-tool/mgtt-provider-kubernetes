#!/bin/bash
set -euo pipefail

# Runs from inside the installed provider dir ($MGTT_PROVIDER_DIR).
# Removes build artifacts so an eventual `mgtt provider install` starts
# from a clean slate.
#
# This hook runs BEFORE mgtt removes the provider directory. Even if
# this script fails, mgtt still removes the directory — uninstall must
# always succeed. We therefore keep cleanup simple and idempotent.

cd "$(dirname "$0")/.."

# Built binary.
rm -f bin/mgtt-provider-kubernetes

# Go build/module cache artifacts local to this provider. Global caches
# (~/.cache/go-build, $GOPATH/pkg/mod) are not ours to clean.
rm -rf bin/

echo "✓ kubernetes provider cleaned up"
