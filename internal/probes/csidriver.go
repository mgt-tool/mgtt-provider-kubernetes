package probes

import (
	"context"
	"strings"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

func registerCSIDriver(r *provider.Registry, c *shell.Client) {
	r.Register("csidriver", map[string]provider.ProbeFn{
		"exists": Exists(c, "csidriver", false),
		"node_count": func(ctx context.Context, req provider.Request) (provider.Result, error) {
			// A CSIDriver is "ready on N nodes" when N CSINode objects list it.
			d, err := KubectlJSON(ctx, c, "get", "csinode")
			if err != nil {
				return provider.Result{}, err
			}
			items, _ := d["items"].([]any)
			count := 0
			for _, it := range items {
				im, _ := it.(map[string]any)
				drivers, _ := walk(im, "spec", "drivers").([]any)
				for _, drv := range drivers {
					dm, _ := drv.(map[string]any)
					if name, _ := dm["name"].(string); strings.EqualFold(name, req.Name) {
						count++
						break
					}
				}
			}
			return provider.IntResult(count), nil
		},
	})
}
