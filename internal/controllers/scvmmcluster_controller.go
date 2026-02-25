/*
Copyright 2024.

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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/pkg/errors"
	infrav1 "github.com/willemm/cluster-api-provider-scvmm/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

const (
	ClusterCreated        = "ClusterCreated"
	ClusterDeletingReason = "ClusterDeleting"

	ClusterFinalizer = "scvmmcluster.finalizers.cluster.x-k8s.io"
)

// ScvmmClusterReconciler reconciles a ScvmmCluster object
type ScvmmClusterReconciler struct {
	client.Client
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmclusters/finalizers,verbs=update

func (r *ScvmmClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, retErr error) {
	log := ctrl.LoggerFrom(ctx).WithValues("scvmmcluster", req.NamespacedName)

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

	// Handle deleted clusters
	if !scvmmCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, scvmmCluster)
	}

	// Add finalizer.  Apparently we should return here to avoid a race condition
	// (Presumably the change/patch will trigger another reconciliation so it continues)
	if !controllerutil.ContainsFinalizer(scvmmCluster, ClusterFinalizer) {
		controllerutil.AddFinalizer(scvmmCluster, ClusterFinalizer)
		return ctrl.Result{}, nil
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, scvmmCluster)
}

func patchScvmmCluster(ctx context.Context, patchHelper *patch.Helper, scvmmCluster *infrav1.ScvmmCluster) error {
	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		scvmmCluster,
		patch.WithOwnedConditions{Conditions: []string{
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
	// Copy failureDomains spec to status
	failureDomains := make([]clusterv1.FailureDomain, 0, len(scvmmCluster.Spec.FailureDomains))
	for key, fd := range scvmmCluster.Spec.FailureDomains {
		failureDomains = append(failureDomains, clusterv1.FailureDomain{
			Name:         key,
			ControlPlane: &fd.ControlPlane,
			Attributes: map[string]string{
				"cloud":     fd.Cloud,
				"hostGroup": fd.HostGroup,
			},
		})
	}
	scvmmCluster.Status.FailureDomains = failureDomains

	// Mark the scvmmCluster ready
	scvmmCluster.Status.Initialization.Provisioned = true
	conditions.Set(scvmmCluster, metav1.Condition{
		Status:  metav1.ConditionTrue,
		Type:    ClusterCreated,
		Reason:  ClusterCreated,
		Message: "Cluster Provisioned",
	})

	return ctrl.Result{}, nil
}

func (r *ScvmmClusterReconciler) reconcileDelete(ctx context.Context, scvmmCluster *infrav1.ScvmmCluster) (ctrl.Result, error) {
	patchHelper, err := patch.NewHelper(scvmmCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	conditions.Set(scvmmCluster, metav1.Condition{
		Status:  metav1.ConditionFalse,
		Type:    ClusterCreated,
		Reason:  ClusterDeletingReason,
		Message: "Cluster being deleted",
	})

	if err := patchScvmmCluster(ctx, patchHelper, scvmmCluster); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to patch ScvmmCluster")
	}

	// We'll probably have to delete some stuff at this point

	// Cluster is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(scvmmCluster, ClusterFinalizer)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScvmmClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := ctrl.LoggerFrom(ctx)
	scheme := mgr.GetScheme()
	clusterToScvmm, err := util.ClusterToTypedObjectsMapper(mgr.GetClient(), &infrav1.ScvmmClusterList{}, scheme)
	if err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.ScvmmCluster{}).
		WithOptions(options).
		WithEventFilter(predicates.ResourceNotPaused(scheme, log)).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(clusterToScvmm),
			builder.WithPredicates(predicates.ClusterUnpausedAndInfrastructureProvisioned(scheme, log)),
		).
		Complete(r)
}
