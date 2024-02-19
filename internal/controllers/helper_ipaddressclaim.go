/*
Copyright 2023 The Kubernetes Authors.

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
	"reflect"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1 "github.com/willemm/cluster-api-provider-scvmm/api/v1alpha1"
)

const (
	IPAddressClaimed clusterv1.ConditionType = "IPAddressClaimed"

	IPAddressClaimsBeingCreatedReason = "IPAddressClaimsBeingCreated"
	WaitingForIPAddressReason         = "WaitingForIPAddress"
	IPAddressInvalidReason            = "IPAddressInvalid"
	IPAddressClaimNotFoundReason      = "IPAddressClaimNotFound"
	IPAddressClaimFinalizer           = "scvmmmachine.finalizers.cluster.x-k8s.io/ip-claim-protection"
)

// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=ipaddressclaims,verbs=get;create;patch;watch;list;update
// +kubebuilder:rbac:groups=ipam.cluster.x-k8s.io,resources=ipaddresses,verbs=get;list;watch

// reconcileIPAddressClaims ensures that ScvmmMachines that are configured with
// .spec.networking.devices.addressFromPools have corresponding IPAddressClaims.
// If claims are fulfilled, it also fills those in in the spec
// TODO: Use the status to store fulfilled claims instead of amending the spec
func (r *ScvmmMachineReconciler) reconcileIPAddressClaims(ctx context.Context, scvmmMachine *infrav1.ScvmmMachine) error {
	totalClaims, claimsCreated := 0, 0
	claimsFulfilled := 0
	log := ctrl.LoggerFrom(ctx)

	var (
		claims  []conditions.Getter
		errList []error
	)

	for devIdx, device := range scvmmMachine.Spec.Networking.Devices {
		var gateway string
		addresses := make([]string, len(device.AddressesFromPools))
		for poolRefIdx, poolRef := range device.AddressesFromPools {
			totalClaims++
			ipAddrClaimName := fmt.Sprintf("%s-%d-%d", scvmmMachine.ObjectMeta.Name, devIdx, poolRefIdx)
			ipAddrClaim := &ipamv1.IPAddressClaim{}
			ipAddrClaimKey := client.ObjectKey{
				Namespace: scvmmMachine.ObjectMeta.Namespace,
				Name:      ipAddrClaimName,
			}

			loopctx := ctrl.LoggerInto(ctx, log.WithValues("IPAddressClaim", klog.KRef(ipAddrClaimKey.Namespace, ipAddrClaimKey.Name)))

			err := r.Client.Get(loopctx, ipAddrClaimKey, ipAddrClaim)
			if err != nil && !apierrors.IsNotFound(err) {
				return errors.Wrapf(err, "failed to get IPAddressClaim %s", klog.KRef(ipAddrClaimKey.Namespace, ipAddrClaimKey.Name))
			}
			ipAddrClaim, created, err := createOrPatchIPAddressClaim(loopctx, r.Client, scvmmMachine, ipAddrClaimName, poolRef)
			if err != nil {
				errList = append(errList, err)
				continue
			}
			if created {
				claimsCreated++
			}
			if ipAddrClaim.Status.AddressRef.Name != "" {
				ipAddr := &ipamv1.IPAddress{}
				ipAddrKey := client.ObjectKey{
					Namespace: scvmmMachine.ObjectMeta.Namespace,
					Name:      ipAddrClaim.Status.AddressRef.Name,
				}
				log.V(1).Info("Get ipaddress", "namespacedname", ipAddrKey)
				if err := r.Client.Get(loopctx, ipAddrKey, ipAddr); err != nil {
					// Should not happen but you never know
					errList = append(errList, err)
					continue
				}
				if gateway != "" && gateway != ipAddr.Spec.Gateway {
					err := fmt.Errorf("Different gateways: %s <> %s", gateway, ipAddr.Spec.Gateway)
					errList = append(errList, err)
					continue
				}
				gateway = ipAddr.Spec.Gateway
				addresses[poolRefIdx] = fmt.Sprintf("%s/%d", ipAddr.Spec.Address, ipAddr.Spec.Prefix)
				claimsFulfilled++
			}

			// Since this is eventually used to calculate the status of the
			// IPAddressClaimed condition for the ScvmmMachine object.
			if conditions.Has(ipAddrClaim, clusterv1.ReadyCondition) {
				claims = append(claims, ipAddrClaim)
			}
		}
		log.V(1).Info("Reconciling addresses, got claims", "gateway", gateway, "addresses", addresses)
		if gateway != "" {
			if gateway != device.Gateway {
				scvmmMachine.Spec.Networking.Devices[devIdx].Gateway = gateway
			}
			if !reflect.DeepEqual(device.IPAddresses, addresses) {
				scvmmMachine.Spec.Networking.Devices[devIdx].IPAddresses = addresses
			}
		}
	}

	if len(errList) > 0 {
		aggregatedErr := kerrors.NewAggregate(errList)
		conditions.MarkFalse(scvmmMachine,
			IPAddressClaimed,
			IPAddressClaimNotFoundReason,
			clusterv1.ConditionSeverityError,
			aggregatedErr.Error())
		return aggregatedErr
	}

	if len(claims) == totalClaims {
		conditions.SetAggregate(scvmmMachine,
			IPAddressClaimed,
			claims,
			conditions.AddSourceRef(),
			conditions.WithStepCounter())
		return nil
	}

	// Fallback logic to calculate the state of the IPAddressClaimed condition
	switch {
	case totalClaims == claimsFulfilled:
		conditions.MarkTrue(scvmmMachine, IPAddressClaimed)
	case claimsFulfilled < totalClaims && claimsCreated > 0:
		conditions.MarkFalse(scvmmMachine, IPAddressClaimed,
			IPAddressClaimsBeingCreatedReason, clusterv1.ConditionSeverityInfo,
			"%d/%d claims being created", claimsCreated, totalClaims)
	case claimsFulfilled < totalClaims && claimsCreated == 0:
		conditions.MarkFalse(scvmmMachine, IPAddressClaimed,
			WaitingForIPAddressReason, clusterv1.ConditionSeverityInfo,
			"%d/%d claims being processed", totalClaims-claimsFulfilled, totalClaims)
	}
	return nil
}

// createOrPatchIPAddressClaim creates/patches an IPAddressClaim object for a device requesting an address
// from an externally managed IPPool. Ensures that the claim has a reference to the cluster of the VM to
// support pausing reconciliation.
// The responsibility of the IP address resolution is handled by an external IPAM provider.
func createOrPatchIPAddressClaim(ctx context.Context, c client.Client, scvmmMachine *infrav1.ScvmmMachine, name string, poolRef corev1.TypedLocalObjectReference) (*ipamv1.IPAddressClaim, bool, error) {
	claim := &ipamv1.IPAddressClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: scvmmMachine.ObjectMeta.Namespace,
		},
	}
	mutateFn := func() (err error) {
		claim.SetOwnerReferences(util.EnsureOwnerRef(
			claim.OwnerReferences,
			metav1.OwnerReference{
				APIVersion: infrav1.GroupVersion.String(),
				Kind:       "ScvmmMachine",
				Name:       scvmmMachine.ObjectMeta.Name,
				UID:        scvmmMachine.ObjectMeta.UID,
			}))

		controllerutil.AddFinalizer(claim, IPAddressClaimFinalizer)

		if claim.Labels == nil {
			claim.Labels = make(map[string]string)
		}
		claim.Labels[clusterv1.ClusterNameLabel] = scvmmMachine.ObjectMeta.Labels[clusterv1.ClusterNameLabel]

		if claim.Annotations == nil {
			claim.Annotations = make(map[string]string)
		}
		claim.Annotations["infrastructure.x-k8s.io/hostname"] = scvmmMachine.Spec.VMName

		claim.Spec.PoolRef.APIGroup = poolRef.APIGroup
		claim.Spec.PoolRef.Kind = poolRef.Kind
		claim.Spec.PoolRef.Name = poolRef.Name
		return nil
	}
	log := ctrl.LoggerFrom(ctx)

	result, err := controllerutil.CreateOrPatch(ctx, c, claim, mutateFn)
	log.V(1).Info("Create or patch ipaddressclaim result", "claim", claim)
	if err != nil {
		return nil, false, errors.Wrap(err, "failed to CreateOrPatch IPAddressClaim")
	}
	switch result {
	case controllerutil.OperationResultCreated:
		log.Info("Created IPAddressClaim")
		return claim, true, nil
	case controllerutil.OperationResultUpdated:
		log.Info("Updated IPAddressClaim")
	case controllerutil.OperationResultNone, controllerutil.OperationResultUpdatedStatus, controllerutil.OperationResultUpdatedStatusOnly:
		log.V(3).Info("No change required for IPAddressClaim", "operationResult", result)
	}
	return claim, false, nil
}

// deleteIPAddressClaims removes the finalizers from the IPAddressClaim objects
// they will be deleted because the owner object (the scvmmmachine) will disappear
func (r *ScvmmMachineReconciler) deleteIPAddressClaims(ctx context.Context, scvmmMachine *infrav1.ScvmmMachine) error {
	log := ctrl.LoggerFrom(ctx)
	for devIdx, device := range scvmmMachine.Spec.Networking.Devices {
		for poolRefIdx := range device.AddressesFromPools {
			// check if claim exists
			ipAddrClaim := &ipamv1.IPAddressClaim{}
			ipAddrClaimName := fmt.Sprintf("%s-%d-%d", scvmmMachine.ObjectMeta.Name, devIdx, poolRefIdx)
			ipAddrClaimKey := client.ObjectKey{
				Namespace: scvmmMachine.ObjectMeta.Namespace,
				Name:      ipAddrClaimName,
			}
			if err := r.Client.Get(ctx, ipAddrClaimKey, ipAddrClaim); err != nil {
				if apierrors.IsNotFound(err) {
					continue
				}
				return errors.Wrapf(err, fmt.Sprintf("failed to get IPAddressClaim %q to remove the finalizer", ipAddrClaimName))
			}

			if controllerutil.RemoveFinalizer(ipAddrClaim, IPAddressClaimFinalizer) {
				log.Info(fmt.Sprintf("Removing finalizer %s", IPAddressClaimFinalizer), "IPAddressClaim", klog.KObj(ipAddrClaim))
				if err := r.Client.Update(ctx, ipAddrClaim); err != nil {
					return errors.Wrapf(err, fmt.Sprintf("failed to update IPAddressClaim %s", klog.KObj(ipAddrClaim)))
				}
			}
		}
	}
	return nil
}

func hasAllIPAddresses(networking *infrav1.Networking) bool {
	for _, device := range networking.Devices {
		if device.Gateway == "" {
			return false
		}
		if len(device.IPAddresses) == 0 {
			return false
		}
		if len(device.IPAddresses) < len(device.AddressesFromPools) {
			return false
		}
		for _, address := range device.IPAddresses {
			if address == "" {
				return false
			}
		}
	}
	return true
}
