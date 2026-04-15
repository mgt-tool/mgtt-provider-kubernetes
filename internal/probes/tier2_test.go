package probes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mgt-tool/mgtt/sdk/provider"
	"github.com/mgt-tool/mgtt/sdk/provider/shell"
)

// fakeKubectl builds a shell.Client whose Exec returns the given JSON blob
// unchanged (after -o json). Used for locking probe behavior without
// running real kubectl.
func fakeKubectl(t *testing.T, response any) *shell.Client {
	t.Helper()
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	c := shell.New("kubectl")
	c.Exec = func(ctx context.Context, args ...string) ([]byte, []byte, error) {
		return raw, nil, nil
	}
	c.Classify = func(stderr string, runErr error) error { return runErr }
	return c
}

func probeOnce(t *testing.T, register func(*provider.Registry, *shell.Client), c *shell.Client,
	req provider.Request) provider.Result {
	t.Helper()
	r := provider.NewRegistry()
	register(r, c)
	res, err := r.Probe(context.Background(), req)
	if err != nil {
		t.Fatalf("probe %+v: %v", req, err)
	}
	return res
}

func TestReplicaset_ReadyReplicas(t *testing.T) {
	c := fakeKubectl(t, map[string]any{
		"status": map[string]any{"readyReplicas": 3},
	})
	res := probeOnce(t, registerReplicaset, c, provider.Request{
		Type: "replicaset", Name: "x", Namespace: "default", Fact: "ready_replicas",
	})
	if res.Value != 3 {
		t.Fatalf("want 3, got %v", res.Value)
	}
}

func TestCronJob_Suspended(t *testing.T) {
	c := fakeKubectl(t, map[string]any{"spec": map[string]any{"suspend": true}})
	res := probeOnce(t, registerCronJob, c, provider.Request{
		Type: "cronjob", Name: "x", Namespace: "default", Fact: "suspended",
	})
	if res.Value != true {
		t.Fatalf("want true, got %v", res.Value)
	}
}

func TestCronJob_LastScheduleAge(t *testing.T) {
	ts := time.Now().Add(-120 * time.Second).UTC().Format(time.RFC3339)
	c := fakeKubectl(t, map[string]any{"status": map[string]any{"lastScheduleTime": ts}})
	res := probeOnce(t, registerCronJob, c, provider.Request{
		Type: "cronjob", Name: "x", Namespace: "default", Fact: "last_schedule_age",
	})
	age, _ := res.Value.(int)
	if age < 118 || age > 125 {
		t.Fatalf("want ~120, got %v", res.Value)
	}
}

func TestJob_ConditionComplete(t *testing.T) {
	c := fakeKubectl(t, map[string]any{"status": map[string]any{"conditions": []any{
		map[string]any{"type": "Complete", "status": "True"},
	}}})
	res := probeOnce(t, registerJob, c, provider.Request{
		Type: "job", Name: "x", Namespace: "default", Fact: "condition_complete",
	})
	if res.Value != true {
		t.Fatalf("want true, got %v", res.Value)
	}
}

func TestNetworkPolicy_IngressRuleCount(t *testing.T) {
	c := fakeKubectl(t, map[string]any{"spec": map[string]any{"ingress": []any{
		map[string]any{}, map[string]any{}, map[string]any{},
	}}})
	res := probeOnce(t, registerNetworkPolicy, c, provider.Request{
		Type: "networkpolicy", Name: "np", Namespace: "default", Fact: "ingress_rule_count",
	})
	if res.Value != 3 {
		t.Fatalf("want 3, got %v", res.Value)
	}
}

func TestIngressClass_Controller(t *testing.T) {
	c := fakeKubectl(t, map[string]any{"spec": map[string]any{"controller": "k8s.io/nginx-ingress"}})
	res := probeOnce(t, registerIngressClass, c, provider.Request{
		Type: "ingressclass", Name: "nginx", Fact: "controller",
	})
	if res.Value != "k8s.io/nginx-ingress" {
		t.Fatalf("got %v", res.Value)
	}
}

func TestPDB_CurrentHealthy(t *testing.T) {
	c := fakeKubectl(t, map[string]any{"status": map[string]any{"currentHealthy": 5}})
	res := probeOnce(t, registerPDB, c, provider.Request{
		Type: "pdb", Name: "x", Namespace: "default", Fact: "current_healthy",
	})
	if res.Value != 5 {
		t.Fatalf("want 5, got %v", res.Value)
	}
}

func TestNamespace_Terminating(t *testing.T) {
	c := fakeKubectl(t, map[string]any{"status": map[string]any{"phase": "Terminating"}})
	res := probeOnce(t, registerNamespace, c, provider.Request{
		Type: "namespace", Name: "old", Fact: "terminating",
	})
	if res.Value != true {
		t.Fatalf("want true, got %v", res.Value)
	}
}

func TestConfigMap_KeyCount_DataAndBinary(t *testing.T) {
	c := fakeKubectl(t, map[string]any{
		"data":       map[string]any{"a": "x", "b": "y"},
		"binaryData": map[string]any{"z": "..."},
	})
	res := probeOnce(t, registerConfigMap, c, provider.Request{
		Type: "configmap", Name: "cm", Namespace: "default", Fact: "key_count",
	})
	if res.Value != 3 {
		t.Fatalf("want 3, got %v", res.Value)
	}
}

func TestSecret_KeyCount_DataOnly(t *testing.T) {
	c := fakeKubectl(t, map[string]any{"data": map[string]any{"a": "xxx", "b": "yyy"}})
	res := probeOnce(t, registerSecret, c, provider.Request{
		Type: "secret", Name: "s", Namespace: "default", Fact: "key_count",
	})
	if res.Value != 2 {
		t.Fatalf("want 2, got %v", res.Value)
	}
}

// Guardrail: the secret probe contract MUST NOT emit any value content.
// A secret probe's Raw field should never echo the base64 data we read.
func TestSecret_RawNeverContainsSecretData(t *testing.T) {
	c := fakeKubectl(t, map[string]any{"data": map[string]any{"password": "c2VjcmV0LXZhbHVl"}})
	res := probeOnce(t, registerSecret, c, provider.Request{
		Type: "secret", Name: "s", Namespace: "default", Fact: "key_count",
	})
	if strings.Contains(res.Raw, "secret-value") || strings.Contains(res.Raw, "c2VjcmV0") {
		t.Fatalf("Raw leaked secret content: %q", res.Raw)
	}
}

func TestServiceAccount_HasIRSA(t *testing.T) {
	c := fakeKubectl(t, map[string]any{"metadata": map[string]any{"annotations": map[string]any{
		"eks.amazonaws.com/role-arn": "arn:aws:iam::111:role/x",
	}}})
	res := probeOnce(t, registerServiceAccount, c, provider.Request{
		Type: "serviceaccount", Name: "sa", Namespace: "default", Fact: "has_irsa",
	})
	if res.Value != true {
		t.Fatalf("want true, got %v", res.Value)
	}
}
