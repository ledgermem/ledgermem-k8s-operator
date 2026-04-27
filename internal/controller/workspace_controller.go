package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ledgermemv1alpha1 "github.com/ledgermem/ledgermem-k8s-operator/api/v1alpha1"
)

// WorkspaceReconciler reconciles a Workspace by calling the LedgerMem
// admin API of the referenced cluster.
type WorkspaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// HTTPClient — overridable in tests.
	HTTPClient *http.Client
}

// +kubebuilder:rbac:groups=ledgermem.io,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ledgermem.io,resources=workspaces/status,verbs=get;update;patch

// Reconcile creates or updates the workspace upstream.
func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("workspace", req.NamespacedName)

	var ws ledgermemv1alpha1.Workspace
	if err := r.Get(ctx, req.NamespacedName, &ws); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Already created — nothing to do for this scaffold reconciler.
	if ws.Status.WorkspaceID != "" {
		return ctrl.Result{}, nil
	}

	cli := r.HTTPClient
	if cli == nil {
		cli = &http.Client{Timeout: 15 * time.Second}
	}

	body, _ := json.Marshal(map[string]any{
		"name":          ws.Spec.Name,
		"slug":          ws.Spec.Slug,
		"plan":          ws.Spec.Plan,
		"retentionDays": ws.Spec.RetentionDays,
	})
	url := fmt.Sprintf("http://%s.%s.svc.cluster.local/v1/admin/workspaces", ws.Spec.ClusterRef, ws.Namespace)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return ctrl.Result{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := cli.Do(httpReq)
	if err != nil {
		logger.Error(err, "call workspace endpoint")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		logger.Info("workspace create non-2xx", "status", resp.StatusCode)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	var out struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	ws.Status.WorkspaceID = out.ID
	ws.Status.Conditions = append(ws.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionTrue, Reason: "Created",
		Message: "workspace created upstream", LastTransitionTime: metav1.Now(),
	})
	if err := r.Status().Update(ctx, &ws); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller.
func (r *WorkspaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ledgermemv1alpha1.Workspace{}).
		Complete(r)
}
