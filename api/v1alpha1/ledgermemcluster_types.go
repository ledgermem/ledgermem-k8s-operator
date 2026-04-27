package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// PostgresSpec describes the Postgres backend the cluster talks to.
type PostgresSpec struct {
	Host       string                   `json:"host"`
	Port       int32                    `json:"port,omitempty"`
	Database   string                   `json:"database"`
	SecretRef  *corev1.SecretKeySelector `json:"secretRef,omitempty"`
	SslMode    string                   `json:"sslMode,omitempty"`
}

// VectorStoreSpec describes the vector store backend (pgvector or pinecone).
type VectorStoreSpec struct {
	// +kubebuilder:validation:Enum=pgvector;pinecone;qdrant
	Provider string                    `json:"provider"`
	IndexName string                   `json:"indexName,omitempty"`
	SecretRef *corev1.SecretKeySelector `json:"secretRef,omitempty"`
}

// LedgerMemClusterSpec defines the desired state of a LedgerMemCluster.
type LedgerMemClusterSpec struct {
	// Image to deploy.
	// +kubebuilder:default="ghcr.io/ledgermem/ledgermem:latest"
	Image string `json:"image,omitempty"`

	// Replicas — number of pods.
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas,omitempty"`

	Postgres    PostgresSpec    `json:"postgres"`
	VectorStore VectorStoreSpec `json:"vectorStore"`

	// Resources — pod resource requests/limits.
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// LedgerMemClusterStatus reflects observed state.
type LedgerMemClusterStatus struct {
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// LedgerMemCluster is a managed LedgerMem deployment.
type LedgerMemCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              LedgerMemClusterSpec   `json:"spec,omitempty"`
	Status            LedgerMemClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LedgerMemClusterList lists clusters.
type LedgerMemClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LedgerMemCluster `json:"items"`
}

// DeepCopyObject — runtime.Object impl.
func (l *LedgerMemCluster) DeepCopyObject() runtime.Object     { return l.deepCopy() }
func (l *LedgerMemClusterList) DeepCopyObject() runtime.Object { return l.deepCopy() }

func (l *LedgerMemCluster) deepCopy() *LedgerMemCluster {
	if l == nil {
		return nil
	}
	out := &LedgerMemCluster{TypeMeta: l.TypeMeta}
	l.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.Spec = l.Spec
	out.Status = *l.Status.DeepCopy()
	return out
}

func (l *LedgerMemClusterList) deepCopy() *LedgerMemClusterList {
	if l == nil {
		return nil
	}
	out := &LedgerMemClusterList{TypeMeta: l.TypeMeta, ListMeta: *l.ListMeta.DeepCopy()}
	out.Items = make([]LedgerMemCluster, len(l.Items))
	for i := range l.Items {
		out.Items[i] = *l.Items[i].deepCopy()
	}
	return out
}

// DeepCopy returns a deep copy of the status.
func (s *LedgerMemClusterStatus) DeepCopy() *LedgerMemClusterStatus {
	if s == nil {
		return nil
	}
	out := *s
	if s.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(s.Conditions))
		copy(out.Conditions, s.Conditions)
	}
	return &out
}
