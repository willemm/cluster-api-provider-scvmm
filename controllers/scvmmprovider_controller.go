/*
Copyright 2021.

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
	infrav1 "github.com/willemm/cluster-api-provider-scvmm/api/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ScvmmProviderReconciler reconciles a ScvmmProvider object
type ScvmmProviderReconciler struct {
	client.Client
	Log logr.Logger
	// Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmproviders,verbs=get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmproviders/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmproviders/finalizers,verbs=update

// This reconcile loop currently just reads the providers into memory for the winrm workers
// Seemed the easiest way to force the workers to reload when the provider changes, without having
// to read them every time
func (r *ScvmmProviderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// log := r.Log.WithValues("scvmmprovider", req.NamespacedName)

	providerRef := infrav1.ScvmmProviderReference{
		Name:      req.NamespacedName.Name,
		Namespace: req.NamespacedName.Namespace,
	}
	scvmmProvider := &infrav1.ScvmmProvider{}
	if err := r.Client.Get(ctx, req.NamespacedName, scvmmProvider); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		delete(winrmProviders, providerRef)
		return ctrl.Result{}, nil
	}

	if !scvmmProvider.DeletionTimestamp.IsZero() {
		delete(winrmProviders, providerRef)
		return ctrl.Result{}, nil
	}
	// TODO: Verify that the scripts work and add status field

	winrmProviders[providerRef] = WinrmProvider{
		Spec:            scvmmProvider.Spec,
		ResourceVersion: scvmmProvider.ObjectMeta.ResourceVersion,
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScvmmProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.ScvmmProvider{}).
		Complete(r)
}
