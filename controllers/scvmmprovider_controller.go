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
	"fmt"
	"os"

	"github.com/go-logr/logr"
	infrav1 "github.com/willemm/cluster-api-provider-scvmm/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
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
	scvmmProvider, err := r.getProvider(ctx, providerRef)
	if err != nil {
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

func (r *ScvmmProviderReconciler) getProvider(ctx context.Context, providerRef infrav1.ScvmmProviderReference) (*infrav1.ScvmmProvider, error) {
	log := r.Log.WithValues("providerref", providerRef)
	provider := &infrav1.ScvmmProvider{}
	if providerRef.Name != "" {
		log.V(1).Info("Fetching provider ref")
		key := client.ObjectKey{Namespace: providerRef.Namespace, Name: providerRef.Name}
		if err := r.Client.Get(ctx, key, provider); err != nil {
			return nil, fmt.Errorf("Failed to get ScvmmProvider: %v", err)
		}
	}
	p := &provider.Spec

	// Set defaults
	if p.ScvmmHost == "" {
		p.ScvmmHost = os.Getenv("SCVMM_HOST")
		if p.ScvmmHost == "" {
			return nil, fmt.Errorf("missing required value ScvmmHost")
		}
	}
	if p.ExecHost == "" {
		p.ExecHost = os.Getenv("SCVMM_EXECHOST")
		if p.ExecHost == "" {
			p.ExecHost = p.ScvmmHost
		}
	}
	if p.ScvmmLibraryISOs == "" {
		p.ScvmmLibraryISOs = os.Getenv("SCVMM_LIBRARY")
		if p.ScvmmLibraryISOs == "" {
			p.ScvmmLibraryISOs = `\\` + p.ScvmmHost + `\MSSCVMMLibrary\ISOs\cloud-init`
		}
	}
	if p.ADServer == "" {
		p.ADServer = os.Getenv("ACTIVEDIRECTORY_SERVER")
	}
	if p.ScvmmSecret != nil {
		log.V(1).Info("Fetching scvmm secret ref", "secret", p.ScvmmSecret)
		creds := &corev1.Secret{}
		key := client.ObjectKey{Namespace: provider.Namespace, Name: p.ScvmmSecret.Name}
		if err := r.Client.Get(ctx, key, creds); err != nil {
			return nil, fmt.Errorf("Failed to get scvmm credential secretref: %v", err)
		}
		if value, ok := creds.Data["username"]; ok {
			p.ScvmmUsername = string(value)
		}
		if value, ok := creds.Data["password"]; ok {
			p.ScvmmPassword = string(value)
		}
	}
	if p.ADSecret != nil {
		log.V(1).Info("Fetching AD secret ref", "secret", p.ADSecret)
		creds := &corev1.Secret{}
		key := client.ObjectKey{Namespace: provider.Namespace, Name: p.ADSecret.Name}
		if err := r.Client.Get(ctx, key, creds); err != nil {
			return nil, fmt.Errorf("Failed to get AD credential secretref: %v", err)
		}
		if value, ok := creds.Data["username"]; ok {
			p.ADUsername = string(value)
		}
		if value, ok := creds.Data["password"]; ok {
			p.ADPassword = string(value)
		}
	}
	if p.ScvmmUsername == "" {
		p.ScvmmUsername = os.Getenv("SCVMM_USERNAME")
	}
	if p.ScvmmPassword == "" {
		p.ScvmmPassword = os.Getenv("SCVMM_PASSWORD")
	}
	if p.ADUsername == "" {
		p.ADUsername = os.Getenv("ACTIVEDIRECTORY_USERNAME")
	}
	if p.ADPassword == "" {
		p.ADPassword = os.Getenv("ACTIVEDIRECTOYR_PASSWORD")
	}
	if p.Env == nil {
		p.Env = make(map[string]string)
	}
	p.Env["SCVMM_USERNAME"] = p.ScvmmUsername
	p.Env["SCVMM_PASSWORD"] = p.ScvmmPassword
	p.Env["ACTIVEDIRECTORY_USERNAME"] = p.ADUsername
	p.Env["ACTIVEDIRECTORY_PASSWORD"] = p.ADPassword
	return provider, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScvmmProviderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Fill default provider (for when it is not filled)
	providerRef := infrav1.ScvmmProviderReference{}
	scvmmProvider, err := r.getProvider(context.TODO(), providerRef)
	if err == nil {
		winrmProviders[providerRef] = WinrmProvider{
			Spec:            scvmmProvider.Spec,
			ResourceVersion: scvmmProvider.ObjectMeta.ResourceVersion,
		}
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.ScvmmProvider{}).
		Complete(r)
}
