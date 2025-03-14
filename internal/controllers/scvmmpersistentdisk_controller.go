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

	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	infrav1 "github.com/willemm/cluster-api-provider-scvmm/api/v1alpha1"
)

// ScvmmPersistentDiskReconciler reconciles a ScvmmPersistentDisk object
type ScvmmPersistentDiskReconciler struct {
	client.Client
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmpersistentdisks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmpersistentdisks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmpersistentdisks/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ScvmmPersistentDiskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(1).Info("Fetching scvmmpersistentdisk")
	disk := &infrav1.ScvmmPersistentDisk{}
	if err := r.Get(ctx, req.NamespacedName, disk); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	_ = disk

	// Check for deletion
	// - Remove disk from scvmm (todo: how are you supposed to do that??)
	// - remove finalizer

	// Add finalizer

	// If not owned by machine, check for machines that want this diskpool

	// Update status of owning diskpool

	// If there are free slots (current < max), check for machines that want this diskpool
	// (could be triggered by maxdisks being increased)

	return ctrl.Result{}, nil

	// ScvmmPersistentDisk is like a statefulset; the naming is sequentially numbered
	// Which means that it is easy to concurrently check for free slots
	// If the maxDisks setting is ever decreased, that means there can be
	// persistentdisk resources that will never be used.
	// (Maybe set error status if there are more persistentdisk resources than maxdisks)

	////// Scvmmmachine flow:
	// When creating disk that references diskpool, or triggered by unowned
	//
	// Get disk from diskpool that doesn't have machine owner and set owner to machine
	//  (retry if setting owner fails)
	//
	// If no free disks, set disk number to largest existing plus one.
	//  if that's greater than maxdisks, do nothing and break
	//  else, create scvmmpersistent disk with that number (if that fails, redo from start)
	//
	// Use persistentdisk resource to add vdd to machine
	//  (if location is blank, create new and set location in resource)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScvmmPersistentDiskReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	log := ctrl.LoggerFrom(ctx)
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.ScvmmPersistentDisk{}).
		WithEventFilter(predicate.And(
			predicates.ResourceNotPaused(log),
		)).
		Complete(r)
}
