package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// ApiKeySpec describes a desired API key. The reconciler creates the key
// against the Mnemo admin API and writes the resulting plaintext token
// into a Secret (SecretName) in the same namespace.
type ApiKeySpec struct {
	WorkspaceRef string   `json:"workspaceRef"`
	Name         string   `json:"name"`
	Scopes       []string `json:"scopes,omitempty"`

	// SecretName — the Secret to write the resulting token to. Defaults to
	// "<apikey-name>-token".
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// ApiKeyStatus reflects observed state.
type ApiKeyStatus struct {
	APIKeyID   string             `json:"apiKeyId,omitempty"`
	SecretName string             `json:"secretName,omitempty"`
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// ApiKey is a Mnemo API key managed by the operator.
type ApiKey struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ApiKeySpec   `json:"spec,omitempty"`
	Status            ApiKeyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ApiKeyList lists API keys.
type ApiKeyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ApiKey `json:"items"`
}

func (a *ApiKey) DeepCopyObject() runtime.Object     { return a.deepCopy() }
func (a *ApiKeyList) DeepCopyObject() runtime.Object { return a.deepCopy() }

func (a *ApiKey) deepCopy() *ApiKey {
	if a == nil {
		return nil
	}
	out := &ApiKey{TypeMeta: a.TypeMeta}
	a.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = a.Spec
	if a.Spec.Scopes != nil {
		out.Spec.Scopes = append([]string(nil), a.Spec.Scopes...)
	}
	out.Status = a.Status
	if a.Status.Conditions != nil {
		out.Status.Conditions = make([]metav1.Condition, len(a.Status.Conditions))
		copy(out.Status.Conditions, a.Status.Conditions)
	}
	return out
}

func (a *ApiKeyList) deepCopy() *ApiKeyList {
	if a == nil {
		return nil
	}
	out := &ApiKeyList{TypeMeta: a.TypeMeta, ListMeta: *a.ListMeta.DeepCopy()}
	out.Items = make([]ApiKey, len(a.Items))
	for i := range a.Items {
		out.Items[i] = *a.Items[i].deepCopy()
	}
	return out
}
