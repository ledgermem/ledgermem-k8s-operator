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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	getmnemov1alpha1 "github.com/getmnemo/getmnemo-k8s-operator/api/v1alpha1"
)

const workspaceFinalizer = "getmnemo.io/workspace-finalizer"

// WorkspaceReconciler reconciles a Workspace by calling the Mnemo
// admin API of the referenced cluster.
type WorkspaceReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// HTTPClient — overridable in tests.
	HTTPClient *http.Client
}

// +kubebuilder:rbac:groups=getmnemo.io,resources=workspaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=getmnemo.io,resources=workspaces/status,verbs=get;update;patch

// Reconcile creates or updates the workspace upstream.
func (r *WorkspaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("workspace", req.NamespacedName)

	var ws getmnemov1alpha1.Workspace
	if err := r.Get(ctx, req.NamespacedName, &ws); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	cli := r.HTTPClient
	if cli == nil {
		cli = &http.Client{Timeout: 15 * time.Second}
	}

	// Handle deletion: tear down the upstream workspace before letting the
	// CR be garbage collected. Without this, deleting the CR orphans the
	// workspace in Mnemo's control plane.
	if !ws.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&ws, workspaceFinalizer) {
			if ws.Status.WorkspaceID != "" {
				url := fmt.Sprintf("http://%s.%s.svc.cluster.local/v1/admin/workspaces/%s", ws.Spec.ClusterRef, ws.Namespace, ws.Status.WorkspaceID)
				delReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
				if err != nil {
					return ctrl.Result{}, err
				}
				resp, err := cli.Do(delReq)
				if err != nil {
					logger.Error(err, "delete upstream workspace")
					return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
				}
				resp.Body.Close()
				// Treat 404 as already gone.
				if resp.StatusCode >= 500 {
					return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
				}
			}
			controllerutil.RemoveFinalizer(&ws, workspaceFinalizer)
			if err := r.Update(ctx, &ws); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Ensure finalizer is set before any external side effect.
	if !controllerutil.ContainsFinalizer(&ws, workspaceFinalizer) {
		controllerutil.AddFinalizer(&ws, workspaceFinalizer)
		if err := r.Update(ctx, &ws); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Already created — nothing to do.
	if ws.Status.WorkspaceID != "" {
		return ctrl.Result{}, nil
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
		For(&getmnemov1alpha1.Workspace{}).
		Complete(r)
}
