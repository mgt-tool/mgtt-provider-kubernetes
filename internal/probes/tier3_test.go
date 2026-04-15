package probes

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

// multiFakeKubectl routes kubectl calls to different response blobs keyed on
// the first meaningful arg after `get`. Used to exercise probes that make
// multiple kubectl calls (rolebinding role_ref_exists, webhook service check).
type multiFakeKubectl struct {
	// responses maps an identifier derived from argv (kind or kind/name) to
	// either a JSON response body or an error to return.
	responses map[string]any
}

func (m *multiFakeKubectl) client() *shell.Client {
	c := shell.New("kubectl")
	c.Exec = func(ctx context.Context, args ...string) ([]byte, []byte, error) {
		key := routeKey(args)
		v, ok := m.responses[key]
		if !ok {
			return nil, []byte("Error from server (NotFound): " + key + " not found"),
				errors.New("exit status 1")
		}
		if err, ok := v.(error); ok {
			return nil, nil, err
		}
		raw, _ := json.Marshal(v)
		return raw, nil, nil
	}
	c.Classify = func(stderr string, runErr error) error {
		if runErr == nil {
			return nil
		}
		if strings.Contains(stderr, "NotFound") {
			return provider.ErrNotFound
		}
		return runErr
	}
	return c
}

// routeKey derives a routing key from argv, ignoring -n flag pairs and -o json.
func routeKey(args []string) string {
	parts := []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-o" {
			i++ // skip "json"
			continue
		}
		if a == "-n" {
			i++ // skip namespace value
			continue
		}
		if a == "get" {
			continue
		}
		parts = append(parts, a)
	}
	return strings.Join(parts, "/")
}

// --- RBAC ------------------------------------------------------------------

func TestRoleBinding_RoleRefExists_True(t *testing.T) {
	m := &multiFakeKubectl{responses: map[string]any{
		"rolebinding/admin-bind": map[string]any{
			"roleRef": map[string]any{"kind": "Role", "name": "admin"},
		},
		"role/admin": map[string]any{},
	}}
	r := provider.NewRegistry()
	registerRoleBinding(r, m.client())
	res, err := r.Probe(context.Background(), provider.Request{
		Type: "rolebinding", Name: "admin-bind", Namespace: "default", Fact: "role_ref_exists",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Value != true {
		t.Fatalf("want true, got %v", res.Value)
	}
}

func TestRoleBinding_RoleRefExists_False_Dangling(t *testing.T) {
	m := &multiFakeKubectl{responses: map[string]any{
		"rolebinding/dangling": map[string]any{
			"roleRef": map[string]any{"kind": "Role", "name": "missing"},
		},
		// "role/missing" deliberately absent → NotFound
	}}
	r := provider.NewRegistry()
	registerRoleBinding(r, m.client())
	res, err := r.Probe(context.Background(), provider.Request{
		Type: "rolebinding", Name: "dangling", Namespace: "default", Fact: "role_ref_exists",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Value != false {
		t.Fatalf("dangling binding should report false, got %v", res.Value)
	}
}

func TestClusterRoleBinding_RejectsRoleRef(t *testing.T) {
	// CRB referencing a namespaced Role is invalid — treat as dangling (false).
	m := &multiFakeKubectl{responses: map[string]any{
		"clusterrolebinding/x": map[string]any{
			"roleRef": map[string]any{"kind": "Role", "name": "bad"},
		},
	}}
	r := provider.NewRegistry()
	registerClusterRoleBinding(r, m.client())
	res, err := r.Probe(context.Background(), provider.Request{
		Type: "clusterrolebinding", Name: "x", Fact: "role_ref_exists",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Value != false {
		t.Fatalf("want false for invalid CRB→Role, got %v", res.Value)
	}
}

// --- Webhooks --------------------------------------------------------------

func TestValidatingWebhook_ServiceAvailable_False_WhenServiceMissing(t *testing.T) {
	m := &multiFakeKubectl{responses: map[string]any{
		"validatingwebhookconfiguration/foo": map[string]any{
			"webhooks": []any{map[string]any{
				"clientConfig": map[string]any{
					"service": map[string]any{"name": "svc", "namespace": "ns"},
				},
			}},
		},
		// service/svc absent → NotFound
	}}
	r := provider.NewRegistry()
	registerValidatingWebhookConfiguration(r, m.client())
	res, err := r.Probe(context.Background(), provider.Request{
		Type: "validatingwebhookconfiguration", Name: "foo", Fact: "service_available",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Value != false {
		t.Fatalf("want false when backend svc missing, got %v", res.Value)
	}
}

func TestValidatingWebhook_ServiceAvailable_True_URLBasedHooks(t *testing.T) {
	// No clientConfig.service → URL-based hook → treated as reachable.
	m := &multiFakeKubectl{responses: map[string]any{
		"validatingwebhookconfiguration/foo": map[string]any{
			"webhooks": []any{map[string]any{
				"clientConfig": map[string]any{"url": "https://example.com/hook"},
			}},
		},
	}}
	r := provider.NewRegistry()
	registerValidatingWebhookConfiguration(r, m.client())
	res, err := r.Probe(context.Background(), provider.Request{
		Type: "validatingwebhookconfiguration", Name: "foo", Fact: "service_available",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Value != true {
		t.Fatalf("URL-based hooks should report true, got %v", res.Value)
	}
}

// --- custom_resource flag parsing -----------------------------------------

func TestCustomResource_RequiresKindOrResource(t *testing.T) {
	c := shell.New("kubectl")
	c.Exec = func(ctx context.Context, args ...string) ([]byte, []byte, error) {
		return []byte(`{}`), nil, nil
	}
	c.Classify = func(stderr string, runErr error) error { return runErr }

	r := provider.NewRegistry()
	registerCustomResource(r, c)

	_, err := r.Probe(context.Background(), provider.Request{
		Type: "custom_resource", Name: "x", Fact: "ready",
		Extra: map[string]string{}, // no kind/resource/api
	})
	if !errors.Is(err, provider.ErrUsage) {
		t.Fatalf("want ErrUsage, got %v", err)
	}
}

func TestCustomResource_ResourceFlag(t *testing.T) {
	c := shell.New("kubectl")
	seenArgs := ""
	c.Exec = func(ctx context.Context, args ...string) ([]byte, []byte, error) {
		seenArgs = strings.Join(args, " ")
		return []byte(`{"status":{"conditions":[{"type":"Ready","status":"True"}]}}`), nil, nil
	}
	c.Classify = func(stderr string, runErr error) error { return runErr }

	r := provider.NewRegistry()
	registerCustomResource(r, c)
	res, err := r.Probe(context.Background(), provider.Request{
		Type: "custom_resource", Name: "w1", Namespace: "default", Fact: "ready",
		Extra: map[string]string{"resource": "widgets.example.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Value != true {
		t.Fatalf("want true, got %v", res.Value)
	}
	if !strings.Contains(seenArgs, "widgets.example.com") {
		t.Fatalf("expected resource in argv, got: %s", seenArgs)
	}
}

// --- Storage ---------------------------------------------------------------

func TestStorageClass_IsDefault(t *testing.T) {
	c := fakeKubectl(t, map[string]any{
		"metadata": map[string]any{"annotations": map[string]any{
			"storageclass.kubernetes.io/is-default-class": "true",
		}},
	})
	res := probeOnce(t, registerStorageClass, c, provider.Request{
		Type: "storageclass", Name: "gp3", Fact: "is_default",
	})
	if res.Value != true {
		t.Fatalf("want true, got %v", res.Value)
	}
}

func TestLimitRange_DefaultCPULimit(t *testing.T) {
	c := fakeKubectl(t, map[string]any{"spec": map[string]any{"limits": []any{
		map[string]any{"type": "Container", "default": map[string]any{
			"cpu": "500m", "memory": "512Mi",
		}},
	}}})
	res := probeOnce(t, registerLimitRange, c, provider.Request{
		Type: "limitrange", Name: "x", Namespace: "default", Fact: "default_cpu_limit",
	})
	if res.Value != "500m" {
		t.Fatalf("want 500m, got %v", res.Value)
	}
}

func TestResourceQuota_CPUUsedString(t *testing.T) {
	c := fakeKubectl(t, map[string]any{"status": map[string]any{
		"used": map[string]any{"requests.cpu": "3500m"},
		"hard": map[string]any{"requests.cpu": "8"},
	}})
	res := probeOnce(t, registerResourceQuota, c, provider.Request{
		Type: "resourcequota", Name: "q", Namespace: "default", Fact: "cpu_used",
	})
	if res.Value != "3500m" {
		t.Fatalf("want 3500m, got %v", res.Value)
	}
}

func TestCSIDriver_NodeCount_CountsMatchingNodes(t *testing.T) {
	m := &multiFakeKubectl{responses: map[string]any{
		"csinode": map[string]any{"items": []any{
			map[string]any{"spec": map[string]any{"drivers": []any{
				map[string]any{"name": "ebs.csi.aws.com"},
				map[string]any{"name": "efs.csi.aws.com"},
			}}},
			map[string]any{"spec": map[string]any{"drivers": []any{
				map[string]any{"name": "ebs.csi.aws.com"},
			}}},
			map[string]any{"spec": map[string]any{"drivers": []any{
				map[string]any{"name": "efs.csi.aws.com"},
			}}},
		}},
	}}
	r := provider.NewRegistry()
	registerCSIDriver(r, m.client())
	res, err := r.Probe(context.Background(), provider.Request{
		Type: "csidriver", Name: "ebs.csi.aws.com", Fact: "node_count",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Value != 2 {
		t.Fatalf("want 2, got %v", res.Value)
	}
}

// --- Extensibility ---------------------------------------------------------

func TestCRD_Established(t *testing.T) {
	c := fakeKubectl(t, map[string]any{"status": map[string]any{"conditions": []any{
		map[string]any{"type": "Established", "status": "True"},
	}}})
	res := probeOnce(t, registerCRD, c, provider.Request{
		Type: "customresourcedefinition", Name: "widgets.example.com", Fact: "established",
	})
	if res.Value != true {
		t.Fatalf("want true, got %v", res.Value)
	}
}

func TestLease_RenewAge(t *testing.T) {
	// 60 seconds old
	c := fakeKubectl(t, map[string]any{"spec": map[string]any{
		"holderIdentity":       "node-1",
		"leaseDurationSeconds": 30,
		"renewTime":            "2026-04-16T00:00:00Z",
	}})
	res := probeOnce(t, registerLease, c, provider.Request{
		Type: "lease", Name: "kube-scheduler", Namespace: "kube-system", Fact: "holder",
	})
	if res.Value != "node-1" {
		t.Fatalf("want node-1, got %v", res.Value)
	}
}
