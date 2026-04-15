package probes

import (
	"context"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerCSIDriver(r *provider.Registry, c *shell.Client) {
	r.Register("csidriver", map[string]provider.ProbeFn{
		"exists": Exists(c, "csidriver", false),
		"node_count": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			// A CSIDriver is "ready on N nodes" when N CSINode objects list
			// it. Push the filter down to kubectl's jsonpath so we don't
			// ship the full CSINode list back to the runner — on large
			// clusters the list can be tens of MB.
			out, err := c.Run(ctx, "get", "csinode",
				"-o", `jsonpath={range .items[?(@.spec.drivers[*].name=="`+req.Name+`")]}x{end}`)
			if err != nil {
				return provider.Result{}, err
			}
			// Count the "x" markers emitted per matching item.
			n := 0
			for _, b := range out {
				if b == 'x' {
					n++
				}
			}
			return provider.IntResult(n), nil
		},
	})
}
