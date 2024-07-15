// +kubebuilder:rbac:groups=serving.kserve.io,resources=clustercachedmodels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=serving.kserve.io,resources=clustercachedmodels/status,verbs=get;update;patch
package cachedmodel

import (
	"context"

	"github.com/go-logr/logr"
	v1alpha1api "github.com/kserve/kserve/pkg/apis/serving/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type CachedModelReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

func (c *CachedModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	c.Log.Info("Hello from model cache controller")
	return reconcile.Result{}, nil
}

func (c *CachedModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1api.ClusterCachedModel{}).
		Complete(c)
}
