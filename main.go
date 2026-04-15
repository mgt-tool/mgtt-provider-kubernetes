// Command mgtt-provider-kubernetes is a kubernetes provider runner binary
// for mgtt. All plumbing (argv parsing, JSON output, exit codes, timeouts,
// size caps, status:not_found translation) lives in the mgtt SDK at
// github.com/mgt-tool/mgtt/sdk/provider — this file only wires probes.
package main

import (
	"github.com/mgt-tool/mgtt-provider-kubernetes/internal/probes"
	"github.com/mgt-tool/mgtt/sdk/provider"
)

func main() {
	r := provider.NewRegistry()
	probes.Register(r)
	provider.Main(r)
}
