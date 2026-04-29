package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// WorkspaceSpec defines a Mnemo workspace, reconciled by calling the
// admin API of the MnemoCluster referenced via ClusterRef.
type WorkspaceSpec struct {
	// ClusterRef is the name of the MnemoCluster in the same namespace.
	ClusterRef string `json:"clusterRef"`

	Name          string `json:"name"`
	Slug          string `json:"slug,omitempty"`
	Plan          string `json:"plan,omitempty"`
	RetentionDays int32  `json:"retentionDays,omitempty"`
}

// WorkspaceStatus reflects observed state.
type WorkspaceStatus struct {
	WorkspaceID string             `json:"workspaceId,omitempty"`
	Conditions  []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Workspace is a Mnemo workspace managed by the operator.
type Workspace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              WorkspaceSpec   `json:"spec,omitempty"`
	Status            WorkspaceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WorkspaceList lists workspaces.
type WorkspaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workspace `json:"items"`
}

func (w *Workspace) DeepCopyObject() runtime.Object     { return w.deepCopy() }
func (w *WorkspaceList) DeepCopyObject() runtime.Object { return w.deepCopy() }

func (w *Workspace) deepCopy() *Workspace {
	if w == nil {
		return nil
	}
	out := &Workspace{TypeMeta: w.TypeMeta}
	w.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = w.Spec
	out.Status = w.Status
	if w.Status.Conditions != nil {
		out.Status.Conditions = make([]metav1.Condition, len(w.Status.Conditions))
		copy(out.Status.Conditions, w.Status.Conditions)
	}
	return out
}

func (w *WorkspaceList) deepCopy() *WorkspaceList {
	if w == nil {
		return nil
	}
	out := &WorkspaceList{TypeMeta: w.TypeMeta, ListMeta: *w.ListMeta.DeepCopy()}
	out.Items = make([]Workspace, len(w.Items))
	for i := range w.Items {
		out.Items[i] = *w.Items[i].deepCopy()
	}
	return out
}
