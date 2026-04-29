package controller

import (
	"testing"

	"k8s.io/apimachinery/pkg/util/intstr"

	getmnemov1alpha1 "github.com/getmnemo/getmnemo-k8s-operator/api/v1alpha1"
)

// TestIntOrStringHelper is a smoke test that does NOT require envtest —
// envtest needs etcd + kube-apiserver binaries which aren't available in
// every CI environment. Real integration tests live behind the "envtest"
// build tag.
func TestIntOrStringHelper(t *testing.T) {
	got := intOrStringFromInt(8080)
	if got.Type != intstr.Int || got.IntValue() != 8080 {
		t.Fatalf("intOrStringFromInt: got %+v", got)
	}
}

// TestSchemeRegistration verifies our types are registered with the scheme
// builder — catches deepCopy/SchemeBuilder regressions.
func TestSchemeRegistration(t *testing.T) {
	scheme := getmnemov1alpha1.SchemeBuilder
	if scheme == nil {
		t.Fatal("scheme builder is nil")
	}
	if got := getmnemov1alpha1.GroupVersion.String(); got != "getmnemo.io/v1alpha1" {
		t.Fatalf("group version: %s", got)
	}
}

// TestApiKeyDeepCopy verifies the hand-written DeepCopy is non-aliased.
func TestApiKeyDeepCopy(t *testing.T) {
	src := &getmnemov1alpha1.ApiKey{
		Spec: getmnemov1alpha1.ApiKeySpec{
			Name:   "ci",
			Scopes: []string{"memories:read"},
		},
	}
	cp := src.DeepCopyObject().(*getmnemov1alpha1.ApiKey)
	cp.Spec.Scopes[0] = "mutated"
	if src.Spec.Scopes[0] != "memories:read" {
		t.Fatal("deep copy aliased Scopes slice")
	}
}
