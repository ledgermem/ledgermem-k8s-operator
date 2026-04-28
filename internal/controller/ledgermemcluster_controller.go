// Package controller holds reconcilers for the ledgermem.io API group.
package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	ledgermemv1alpha1 "github.com/ledgermem/ledgermem-k8s-operator/api/v1alpha1"
)

const (
	defaultImage = "ghcr.io/ledgermem/ledgermem:latest"
	httpPort     = int32(8080)
)

// LedgerMemClusterReconciler reconciles a LedgerMemCluster object.
type LedgerMemClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ledgermem.io,resources=ledgermemclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ledgermem.io,resources=ledgermemclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;persistentvolumeclaims;secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile brings the cluster Deployment + Service in line with the spec.
func (r *LedgerMemClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("ledgermemcluster", req.NamespacedName)

	var cluster ledgermemv1alpha1.LedgerMemCluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if err := r.reconcileDeployment(ctx, &cluster); err != nil {
		logger.Error(err, "reconcile deployment")
		return ctrl.Result{}, err
	}
	if err := r.reconcileService(ctx, &cluster); err != nil {
		logger.Error(err, "reconcile service")
		return ctrl.Result{}, err
	}

	// Sync ReadyReplicas from the Deployment back into status. Surface the
	// error so the controller requeues — silently dropping it leaves the CR
	// reporting a stale ReadyReplicas forever when the apiserver returns a
	// transient conflict / 5xx.
	var dep appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}, &dep); err == nil {
		if cluster.Status.ReadyReplicas != dep.Status.ReadyReplicas {
			cluster.Status.ReadyReplicas = dep.Status.ReadyReplicas
			if err := r.Status().Update(ctx, &cluster); err != nil {
				if apierrors.IsConflict(err) {
					return ctrl.Result{Requeue: true}, nil
				}
				return ctrl.Result{}, fmt.Errorf("update status: %w", err)
			}
		}
	}
	return ctrl.Result{}, nil
}

func (r *LedgerMemClusterReconciler) reconcileDeployment(ctx context.Context, c *ledgermemv1alpha1.LedgerMemCluster) error {
	image := c.Spec.Image
	if image == "" {
		image = defaultImage
	}
	replicas := c.Spec.Replicas
	if replicas == 0 {
		replicas = 2
	}
	res := c.Spec.Resources
	if len(res.Requests) == 0 {
		res.Requests = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("100m"),
			corev1.ResourceMemory: resource.MustParse("256Mi"),
		}
	}
	labels := map[string]string{"app": "ledgermem", "instance": c.Name}

	desired := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: c.Name, Namespace: c.Namespace},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, desired, func() error {
		desired.Labels = labels
		desired.Spec.Replicas = &replicas
		desired.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
		desired.Spec.Template = corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{Labels: labels},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:      "ledgermem",
					Image:     image,
					Resources: res,
					Ports:     []corev1.ContainerPort{{ContainerPort: httpPort, Name: "http"}},
					Env: []corev1.EnvVar{
						{Name: "LEDGERMEM_DB_HOST", Value: c.Spec.Postgres.Host},
						{Name: "LEDGERMEM_DB_NAME", Value: c.Spec.Postgres.Database},
						{Name: "LEDGERMEM_VECTOR_PROVIDER", Value: c.Spec.VectorStore.Provider},
					},
				}},
			},
		}
		return controllerutil.SetControllerReference(c, desired, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("apply deployment: %w", err)
	}
	return nil
}

func (r *LedgerMemClusterReconciler) reconcileService(ctx context.Context, c *ledgermemv1alpha1.LedgerMemCluster) error {
	labels := map[string]string{"app": "ledgermem", "instance": c.Name}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: c.Name, Namespace: c.Namespace},
	}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, svc, func() error {
		svc.Spec.Selector = labels
		svc.Spec.Ports = []corev1.ServicePort{{
			Name:       "http",
			Port:       80,
			TargetPort: intOrStringFromInt(int(httpPort)),
		}}
		return controllerutil.SetControllerReference(c, svc, r.Scheme)
	})
	if err != nil {
		return fmt.Errorf("apply service: %w", err)
	}
	return nil
}

// SetupWithManager registers the controller with the manager.
func (r *LedgerMemClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ledgermemv1alpha1.LedgerMemCluster{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
