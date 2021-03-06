/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"

	infrav1 "github.com/willemm/cluster-api-provider-scvmm/api/v1alpha3"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
)

const (
	ClusterCreated        clusterv1.ConditionType = "ClusterCreated"
	ClusterDeletingReason                         = "ClusterDeleting"

	ClusterFinalizer = "scvmmcluster.finalizers.cluster.x-k8s.io"
)

// ScvmmClusterReconciler reconciles a ScvmmCluster object
type ScvmmClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmclusters/status,verbs=get;update;patch

func (r *ScvmmClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, retErr error) {
	ctx := context.Background()
	log := r.Log.WithValues("scvmmcluster", req.NamespacedName)

	// Fetch the ScvmmCluster instance
	scvmmCluster := &infrav1.ScvmmCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, scvmmCluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, scvmmCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("Waiting for Cluster Controller to set OwnerRef on ScvmmCluster")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	patchHelper, err := patch.NewHelper(scvmmCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		if err := patchScvmmCluster(ctx, patchHelper, scvmmCluster); err != nil {
			log.Error(err, "failed to patch ScvmmCluster")
			if retErr == nil {
				retErr = err
			}
		}
	}()

	// Add finalizer.  Apparently we should return here to avoid a race condition
	// (Presumably the change/patch will trigger another reconciliation so it continues)
	if !controllerutil.ContainsFinalizer(scvmmCluster, ClusterFinalizer) {
		controllerutil.AddFinalizer(scvmmCluster, ClusterFinalizer)
		return ctrl.Result{}, nil
	}

	// Handle deleted clusters
	if !scvmmCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, scvmmCluster)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, scvmmCluster)
}

func patchScvmmCluster(ctx context.Context, patchHelper *patch.Helper, scvmmCluster *infrav1.ScvmmCluster) error {
	// Always update the readyCondition by summarizing the state of other conditions.
	conditions.SetSummary(scvmmCluster,
		conditions.WithConditions(
			ClusterCreated,
		),
		conditions.WithStepCounterIf(scvmmCluster.ObjectMeta.DeletionTimestamp.IsZero()),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		scvmmCluster,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			ClusterCreated,
		}},
	)
}

func (r *ScvmmClusterReconciler) reconcileNormal(ctx context.Context, scvmmCluster *infrav1.ScvmmCluster) (ctrl.Result, error) {
	// We have to get some kind of endpoint IP thing going
	/*
		scvmmCluster.Spec.ControlPlaneEndpoint = &infrav1.APIEndpoint{
			Host: "localhost",
			Port: 6443,
		}
	*/

	// Mark the scvmmCluster ready
	scvmmCluster.Status.Ready = true
	conditions.MarkTrue(scvmmCluster, ClusterCreated)

	return ctrl.Result{}, nil
}

func (r *ScvmmClusterReconciler) reconcileDelete(ctx context.Context, scvmmCluster *infrav1.ScvmmCluster) (ctrl.Result, error) {
	patchHelper, err := patch.NewHelper(scvmmCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	conditions.MarkFalse(scvmmCluster, ClusterCreated, ClusterDeletingReason, clusterv1.ConditionSeverityInfo, "")
	if err := patchScvmmCluster(ctx, patchHelper, scvmmCluster); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to patch ScvmmCluster")
	}

	// We'll probably have to delete some stuff at this point

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(scvmmCluster, ClusterFinalizer)

	return ctrl.Result{}, nil
}

func (r *ScvmmClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.ScvmmCluster{}).
		WithEventFilter(predicates.ResourceNotPaused(r.Log)).
		Build(r)
	if err != nil {
		return err
	}
	return c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: util.ClusterToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("ScvmmCluster")),
		},
		predicates.ClusterUnpaused(r.Log),
	)
}
