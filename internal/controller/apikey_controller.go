package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	getmnemov1alpha1 "github.com/getmnemo/getmnemo-k8s-operator/api/v1alpha1"
)

// ApiKeyReconciler creates a key against the Mnemo admin API and writes
// the resulting plaintext token into a Secret in the same namespace.
type ApiKeyReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	HTTPClient *http.Client
}

// +kubebuilder:rbac:groups=getmnemo.io,resources=apikeys,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=getmnemo.io,resources=apikeys/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is the API key reconciler entrypoint.
func (r *ApiKeyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("apikey", req.NamespacedName)

	var key getmnemov1alpha1.ApiKey
	if err := r.Get(ctx, req.NamespacedName, &key); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if key.Status.APIKeyID != "" {
		return ctrl.Result{}, nil
	}

	// Resolve workspace id via the referenced Workspace CR.
	var ws getmnemov1alpha1.Workspace
	if err := r.Get(ctx, types.NamespacedName{Name: key.Spec.WorkspaceRef, Namespace: key.Namespace}, &ws); err != nil {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, client.IgnoreNotFound(err)
	}
	if ws.Status.WorkspaceID == "" {
		// Workspace not yet provisioned upstream — try again soon.
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	cli := r.HTTPClient
	if cli == nil {
		cli = &http.Client{Timeout: 15 * time.Second}
	}
	body, _ := json.Marshal(map[string]any{
		"workspaceId": ws.Status.WorkspaceID,
		"name":        key.Spec.Name,
		"scopes":      key.Spec.Scopes,
	})
	url := fmt.Sprintf("http://%s.%s.svc.cluster.local/v1/admin/api-keys", ws.Spec.ClusterRef, ws.Namespace)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return ctrl.Result{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := cli.Do(httpReq)
	if err != nil {
		logger.Error(err, "call api-keys endpoint")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	defer resp.Body.Close()
	// 4xx (except 429) is a permanent error — bad scopes, missing workspace,
	// auth failure. Requeueing forever burns API quota and never recovers
	// without user intervention. Mark Failed and stop. 429 + 5xx are
	// transient: requeue with backoff via RequeueAfter.
	if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
		key.Status.Conditions = append(key.Status.Conditions, metav1.Condition{
			Type: "Ready", Status: metav1.ConditionFalse, Reason: "PermanentError",
			Message:            fmt.Sprintf("api returned %d — fix the spec and re-create", resp.StatusCode),
			LastTransitionTime: metav1.Now(),
		})
		_ = r.Status().Update(ctx, &key)
		return ctrl.Result{}, nil
	}
	if resp.StatusCode >= 400 {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}
	var out struct {
		ID     string `json:"id"`
		Secret string `json:"secret"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return ctrl.Result{}, err
	}

	// Write secret.
	secretName := key.Spec.SecretName
	if secretName == "" {
		secretName = key.Name + "-token"
	}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: key.Namespace}}
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, secret, func() error {
		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		secret.Data["token"] = []byte(out.Secret)
		return controllerutil.SetControllerReference(&key, secret, r.Scheme)
	}); err != nil {
		return ctrl.Result{}, err
	}

	key.Status.APIKeyID = out.ID
	key.Status.SecretName = secretName
	key.Status.Conditions = append(key.Status.Conditions, metav1.Condition{
		Type: "Ready", Status: metav1.ConditionTrue, Reason: "Created",
		Message: "api key created and stored in secret", LastTransitionTime: metav1.Now(),
	})
	// Re-fetch + retry on conflict so a transient apiserver conflict during
	// status update doesn't abort reconciliation, leaving us with a created
	// upstream key but no APIKeyID recorded — which would cause the next
	// reconcile to issue a *second* key.
	if err := r.Status().Update(ctx, &key); err != nil {
		if apierrors.IsConflict(err) {
			var fresh getmnemov1alpha1.ApiKey
			if getErr := r.Get(ctx, req.NamespacedName, &fresh); getErr != nil {
				return ctrl.Result{}, getErr
			}
			fresh.Status = key.Status
			if updErr := r.Status().Update(ctx, &fresh); updErr != nil {
				return ctrl.Result{}, updErr
			}
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager registers the controller.
func (r *ApiKeyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&getmnemov1alpha1.ApiKey{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}
