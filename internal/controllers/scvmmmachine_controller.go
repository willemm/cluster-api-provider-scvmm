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
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1alpha1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/pkg/errors"
	infrav1 "github.com/willemm/cluster-api-provider-scvmm/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const (
	// Creation started
	VmCreated clusterv1.ConditionType = "VmCreated"
	// VM running
	VmRunning clusterv1.ConditionType = "VmRunning"

	// Cluster-Api related statuses
	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"
	WaitingForControlPlaneAvailableReason = "WaitingForControlplaneAvailable"
	WaitingForBootstrapDataReason         = "WaitingForBootstrapData"
	WaitingForOwnerReason                 = "WaitingForOwner"
	ClusterNotAvailableReason             = "ClusterNotAvailable"
	ProviderNotAvailableReason            = "ProviderNotAvailable"
	MissingClusterReason                  = "MissingCluster"

	VmCreatingReason = "VmCreating"
	VmUpdatingReason = "VmUpdating"
	VmStartingReason = "VmStarting"
	VmDeletingReason = "VmDeleting"
	VmRunningReason  = "VmRunning"
	VmFailedReason   = "VmFailed"

	MachineFinalizer = "scvmmmachine.finalizers.cluster.x-k8s.io"
)

// Are global variables bad? Dunno, this one seems fine because it caches an env var
var (
	ExtraDebug bool = false
)

// ScvmmMachineReconciler reconciles a ScvmmMachine object
type ScvmmMachineReconciler struct {
	client.Client
	recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmmachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmmachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmmachines/finalizers,verbs=update
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmproviders,verbs=get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmclusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmnamepools,verbs=get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmnamepools/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;machines,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=events;,verbs=create;patch

func (r *ScvmmMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(1).Info("Fetching scvmmmachine")
	// Fetch the instance
	scvmmMachine := &infrav1.ScvmmMachine{}
	if err := r.Get(ctx, req.NamespacedName, scvmmMachine); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Workaround bug in patchhelper
	if scvmmMachine.Spec.Disks != nil {
		for i, d := range scvmmMachine.Spec.Disks {
			if d.Size == nil {
				scvmmMachine.Spec.Disks[i].Size = resource.NewQuantity(0, resource.BinarySI)
			}
		}
	}

	patchHelper, err := patch.NewHelper(scvmmMachine, r)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "Get patchhelper")
	}

	if !scvmmMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, patchHelper, scvmmMachine)
	}

	var cluster *clusterv1.Cluster
	var machine *clusterv1.Machine
	// If the user provides a cloudInit section in the machine, assume it's a standalone machine
	// Otherwise get the owning machine, cluster, etc.
	if scvmmMachine.Spec.CloudInit == nil {
		log.V(1).Info("Fetching machine")
		// Fetch the Machine.
		machine, err = util.GetOwnerMachine(ctx, r.Client, scvmmMachine.ObjectMeta)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "Get owner machine")
		}
		if machine == nil {
			log.Info("Waiting for Machine Controller to set OwnerRef on ScvmmMachine")
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, nil, VmCreated, WaitingForOwnerReason, "")
		}

		log = log.WithValues("machine", machine.Name)

		log.V(1).Info("Fetching cluster")
		// Fetch the Cluster.
		cluster, err = util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
		if err != nil {
			log.Info("ScvmmMachine owner Machine is missing cluster label or cluster does not exist")
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, MissingClusterReason, "ScvmmMachine owner Machine is missing cluster label or cluster does not exist")
		}
		if cluster == nil {
			log.Info(fmt.Sprintf("Please associate this machine with a cluster using the label %s: <name of cluster>", clusterv1.ClusterNameLabel))
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, nil, VmCreated, MissingClusterReason, "Please associate this machine with a cluster using the label %s: <name of cluster>", clusterv1.ClusterNameLabel)
		}

		log = log.WithValues("cluster", cluster.Name)

		// Copy cluster provider ref to machine because the cluster can go away on deletion
		// and then we wouldn't have access to it any more
		if scvmmMachine.Spec.ProviderRef == nil {
			// Fetch the Scvmm Cluster to get the providerRef.
			scvmmClusterName := client.ObjectKey{
				Namespace: cluster.Spec.InfrastructureRef.Namespace,
				Name:      cluster.Spec.InfrastructureRef.Name,
			}
			log.V(1).Info("Fetching scvmmcluster", "scvmmClusterName", scvmmClusterName)
			scvmmCluster := &infrav1.ScvmmCluster{}
			if err := r.Client.Get(ctx, scvmmClusterName, scvmmCluster); err != nil {
				log.Info("ScvmmCluster is not available yet", "error", err)
				return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, nil, VmCreated, ClusterNotAvailableReason, "")
			}
			scvmmMachine.Spec.ProviderRef = scvmmCluster.Spec.ProviderRef
		}

		// Check if the infrastructure is ready, otherwise return and wait for the cluster object to be updated
		if !cluster.Status.InfrastructureReady {
			log.Info("Waiting for ScvmmCluster Controller to create cluster infrastructure")
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, nil, VmCreated, WaitingForClusterInfrastructureReason, "")
		}
	}

	log.V(1).Info("Check finalizer")
	if !controllerutil.ContainsFinalizer(scvmmMachine, MachineFinalizer) {
		log.V(1).Info("Add finalizer")
		controllerutil.AddFinalizer(scvmmMachine, MachineFinalizer)
		if err := patchScvmmMachine(ctx, patchHelper, scvmmMachine); err != nil {
			log.Error(err, "Failed to patch scvmmMachine", "scvmmmachine", scvmmMachine)
			return ctrl.Result{}, err
		}
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, patchHelper, cluster, machine, scvmmMachine)
}

func baseName(path string) string {
	return path[strings.LastIndexAny(path, "\\/")+1:]
}

func (r *ScvmmMachineReconciler) reconcileNormal(ctx context.Context, patchHelper *patch.Helper, cluster *clusterv1.Cluster, machine *clusterv1.Machine, scvmmMachine *infrav1.ScvmmMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("scvmmmachine", scvmmMachine.Name)
	ctx = ctrl.LoggerInto(ctx, log)

	log.Info("Doing reconciliation of ScvmmMachine")
	vm, err := r.getVM(ctx, scvmmMachine)
	if err != nil {
		return ctrl.Result{}, err
	}
	if vm.Id == "" {
		return r.createVM(ctx, patchHelper, scvmmMachine)
	}

	// Create IPAddressClaims after we create the VM because we need vm name
	if err := r.reconcileIPAddressClaims(ctx, scvmmMachine); err != nil {
		return ctrl.Result{}, err
	}

	if vm.Status == "UnderCreation" {
		log.V(1).Info("Creating, Requeue in 15 seconds")
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 15, nil, VmCreated, VmCreatingReason, "")
	}
	log.V(1).Info("Machine is there, fill in status")
	conditions.MarkTrue(scvmmMachine, VmCreated)
	if vm.Status == "PowerOff" {
		if err := r.addVMSpec(ctx, patchHelper, scvmmMachine); err != nil {
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, VmFailedReason, "Failed calling add spec function")
		}
		if vmNeedsExpandDisks(scvmmMachine, vm) {
			return r.expandDisks(ctx, patchHelper, scvmmMachine)
		}

		log.V(1).Info("Get provider")
		provider, err := getProvider(scvmmMachine.Spec.ProviderRef)
		if err != nil {
			log.Error(err, "Failed to get provider")
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, ProviderNotAvailableReason, "")
		}
		if !hasAllIPAddresses(scvmmMachine.Spec.Networking) {
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, nil, VmCreated, WaitingForIPAddressReason, "")
		}

		isoPath := provider.ScvmmLibraryISOs + "\\" + scvmmMachine.Spec.VMName + "-cloud-init.iso"
		if vmNeedsISO(isoPath, scvmmMachine, vm) {
			return r.addISOToVM(ctx, patchHelper, cluster, machine, provider, scvmmMachine, vm, isoPath)
		}
		if (scvmmMachine.Spec.Tag != "" && vm.Tag != scvmmMachine.Spec.Tag) || !equalStringMap(scvmmMachine.Spec.CustomProperty, vm.CustomProperty) {
			return r.setVMProperties(ctx, patchHelper, scvmmMachine)
		}
		return r.startVM(ctx, patchHelper, cluster, machine, provider, scvmmMachine)
	}
	// Support changing properties or tags
	if (scvmmMachine.Spec.Tag != "" && vm.Tag != scvmmMachine.Spec.Tag) || !equalStringMap(scvmmMachine.Spec.CustomProperty, vm.CustomProperty) {
		return r.setVMProperties(ctx, patchHelper, scvmmMachine)
	}

	// Wait for machine to get running state
	if vm.Status != "Running" {
		log.V(1).Info("Not running, Requeue in 15 seconds")
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 15, nil, VmRunning, VmStartingReason, "")
	}
	return r.getVMInfo(ctx, patchHelper, scvmmMachine, vm)
}

func (r *ScvmmMachineReconciler) getVM(ctx context.Context, scvmmMachine *infrav1.ScvmmMachine) (VMResult, error) {
	log := ctrl.LoggerFrom(ctx)
	if scvmmMachine.Spec.Id == "" {
		return VMResult{}, nil
	}
	log.V(1).Info("Running GetVM", "Id", scvmmMachine.Spec.Id)
	vm, err := sendWinrmCommand(log, scvmmMachine.Spec.ProviderRef, "GetVM -Id '%s'",
		escapeSingleQuotes(scvmmMachine.Spec.Id))
	if err != nil {
		r.recorder.Eventf(scvmmMachine, corev1.EventTypeWarning, "GetVM", "%v", err)
		return VMResult{}, errors.Wrap(err, "failed to get vm")
	}
	if vm.Id != scvmmMachine.Spec.Id {
		// Sanity check
		return VMResult{}, fmt.Errorf("GetVM returned VM with different ID (%s <> %s)", vm.Id, scvmmMachine.Spec.Id)
	}
	if vm.VMId != "" {
		scvmmMachine.Spec.ProviderID = "scvmm://" + vm.VMId
	}
	scvmmMachine.Status.Ready = (vm.Status == "Running")
	scvmmMachine.Status.VMStatus = vm.Status
	scvmmMachine.Status.BiosGuid = vm.BiosGuid
	scvmmMachine.Status.CreationTime = vm.CreationTime
	scvmmMachine.Status.ModifiedTime = vm.ModifiedTime
	return vm, nil
}

func (r *ScvmmMachineReconciler) createVM(ctx context.Context, patchHelper *patch.Helper, scvmmMachine *infrav1.ScvmmMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	spec := scvmmMachine.Spec
	vmName := spec.VMName
	if vmName == "" {
		if spec.VMNameFromPool != nil {
			vmName, err := r.generateVMName(ctx, scvmmMachine)
			if err != nil {
				return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, VmFailedReason, "Failed generate vmname")
			}
			scvmmMachine.Spec.VMName = vmName
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, nil, VmCreated, VmCreatingReason, "Set VMName %s", vmName)
		} else {
			vmName = scvmmMachine.ObjectMeta.Name
		}
	}
	diskjson, err := makeDisksJSON(spec.Disks)
	if err != nil {
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, errors.Wrap(err, "Failed to serialize disks"), VmCreated, VmFailedReason, "Failed to create vm")
	}
	optionsjson, err := json.Marshal(spec.VMOptions)
	if err != nil {
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, errors.Wrap(err, "Failed to serialize vmoptions"), VmCreated, VmFailedReason, "Failed to create vm")
	}
	networkjson, err := json.Marshal(spec.Networking.Devices)
	if err != nil {
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, errors.Wrap(err, "Failed to serialize networking"), VmCreated, VmFailedReason, "Failed to create vm")
	}
	memoryFixed := int64(-1)
	memoryMin := int64(-1)
	memoryMax := int64(-1)
	memoryBuffer := -1
	if spec.Memory != nil {
		memoryFixed = spec.Memory.Value() / 1024 / 1024
	}
	if spec.DynamicMemory != nil {
		if spec.DynamicMemory.Minimum != nil {
			memoryMin = spec.DynamicMemory.Minimum.Value() / 1024 / 1024
		}
		if spec.DynamicMemory.Maximum != nil {
			memoryMax = spec.DynamicMemory.Maximum.Value() / 1024 / 1024
		}
		if spec.DynamicMemory.BufferPercentage != nil {
			memoryBuffer = *spec.DynamicMemory.BufferPercentage
		}
	}
	vm, err := sendWinrmCommand(log, spec.ProviderRef, "CreateVM -Cloud '%s' -HostGroup '%s' -VMName '%s' -VMTemplate '%s' -Memory %d -MemoryMin %d -MemoryMax %d -MemoryBuffer %d -CPUCount %d -Disks '%s' -NetworkDevices '%s' -HardwareProfile '%s' -OperatingSystem '%s' -AvailabilitySet '%s' -VMOptions '%s'",
		escapeSingleQuotes(spec.Cloud),
		escapeSingleQuotes(spec.HostGroup),
		escapeSingleQuotes(vmName),
		escapeSingleQuotes(spec.VMTemplate),
		memoryFixed,
		memoryMin,
		memoryMax,
		memoryBuffer,
		spec.CPUCount,
		escapeSingleQuotes(string(diskjson)),
		escapeSingleQuotes(string(networkjson)),
		escapeSingleQuotes(spec.HardwareProfile),
		escapeSingleQuotes(spec.OperatingSystem),
		escapeSingleQuotes(spec.AvailabilitySet),
		escapeSingleQuotes(string(optionsjson)),
	)
	if err != nil {
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, VmFailedReason, "Failed to create vm")
	}

	log.V(1).Info("Fill in status")
	scvmmMachine.Spec.VMName = vmName
	if vm.VMId != "" {
		scvmmMachine.Spec.ProviderID = "scvmm://" + vm.VMId
	}
	scvmmMachine.Spec.Id = vm.Id
	scvmmMachine.Status.Ready = false
	scvmmMachine.Status.VMStatus = vm.Status
	scvmmMachine.Status.BiosGuid = vm.BiosGuid
	scvmmMachine.Status.CreationTime = vm.CreationTime
	scvmmMachine.Status.ModifiedTime = vm.ModifiedTime
	return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 10, nil, VmCreated, VmCreatingReason, "Creating VM %s", vmName)
}

func (r *ScvmmMachineReconciler) setVMProperties(ctx context.Context, patchHelper *patch.Helper, scvmmMachine *infrav1.ScvmmMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	custompropertyjson, err := json.Marshal(scvmmMachine.Spec.CustomProperty)
	if err == nil {
		_, err = sendWinrmCommand(log, scvmmMachine.Spec.ProviderRef, "SetVMProperties -ID '%s' -CustomProperty '%s' -Tag '%s'",
			escapeSingleQuotes(scvmmMachine.Spec.Id),
			escapeSingleQuotes(string(custompropertyjson)),
			escapeSingleQuotes(scvmmMachine.Spec.Tag),
		)
	}
	if err != nil {
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, VmFailedReason, "Failed to set vm properties")
	}
	return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 10, nil, VmCreated, VmUpdatingReason, "Setting properties")
}

func vmNeedsISO(isoPath string, scvmmMachine *infrav1.ScvmmMachine, vm VMResult) bool {
	// If there is an empty cloudinit section, don't add an iso
	if scvmmMachine.Spec.CloudInit != nil {
		if scvmmMachine.Spec.CloudInit.UserData == "" &&
			scvmmMachine.Spec.CloudInit.MetaData == "" &&
			scvmmMachine.Spec.CloudInit.NetworkConfig == "" {
			return false
		}
	}
	for _, iso := range vm.ISOs {
		if baseName(iso.SharePath) == baseName(isoPath) {
			return false
		}
	}
	return true
}

func (r *ScvmmMachineReconciler) addVMSpec(ctx context.Context, patchHelper *patch.Helper, scvmmMachine *infrav1.ScvmmMachine) error {
	log := ctrl.LoggerFrom(ctx)
	newspec, err := sendWinrmSpecCommand(log, scvmmMachine.Spec.ProviderRef, "AddVMSpec", scvmmMachine)
	if err != nil {
		return err
	}
	log.V(1).Info("AddVMSpec result", "newspec", newspec)
	if newspec.CopyNonZeroTo(&scvmmMachine.Spec) {
		if err := patchScvmmMachine(ctx, patchHelper, scvmmMachine); err != nil {
			log.Error(err, "Failed to patch scvmmMachine", "scvmmmachine", scvmmMachine)
			return err
		}
	}
	return nil
}

func vmNeedsExpandDisks(scvmmMachine *infrav1.ScvmmMachine, vm VMResult) bool {
	for i, d := range scvmmMachine.Spec.Disks {
		// For rounding errors
		if d.Size != nil && vm.VirtualDisks[i].MaximumSize < (d.Size.Value()-1024*1024) {
			return true
		}
	}
	return false
}

func (r *ScvmmMachineReconciler) expandDisks(ctx context.Context, patchHelper *patch.Helper, scvmmMachine *infrav1.ScvmmMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	spec := scvmmMachine.Spec
	diskjson, err := makeDisksJSON(spec.Disks)
	var vm VMResult
	if err == nil {
		vm, err = sendWinrmCommand(log, scvmmMachine.Spec.ProviderRef, "ExpandVMDisks -ID '%s' -Disks '%s'",
			escapeSingleQuotes(scvmmMachine.Spec.Id),
			escapeSingleQuotes(string(diskjson)))
	}
	if err != nil {
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, VmFailedReason, "Failed to expand disks")
	}
	scvmmMachine.Status.Ready = false
	scvmmMachine.Status.VMStatus = vm.Status
	scvmmMachine.Status.BiosGuid = vm.BiosGuid
	scvmmMachine.Status.CreationTime = vm.CreationTime
	scvmmMachine.Status.ModifiedTime = vm.ModifiedTime
	return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 10, nil, VmCreated, VmUpdatingReason, "Updating Disks")
}

func (r *ScvmmMachineReconciler) addISOToVM(ctx context.Context, patchHelper *patch.Helper, cluster *clusterv1.Cluster, machine *clusterv1.Machine, provider *infrav1.ScvmmProviderSpec, scvmmMachine *infrav1.ScvmmMachine, vm VMResult, isoPath string) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	var bootstrapData, metaData, networkConfig []byte
	var err error
	if machine != nil {
		if machine.Spec.Bootstrap.DataSecretName == nil {
			if !util.IsControlPlaneMachine(machine) && !conditions.IsTrue(cluster, clusterv1.ControlPlaneInitializedCondition) {
				log.Info("Waiting for the control plane to be initialized")
				return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, nil, VmCreated, WaitingForControlPlaneAvailableReason, "")
			}
			log.Info("Waiting for the Bootstrap provider controller to set bootstrap data")
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, nil, VmCreated, WaitingForBootstrapDataReason, "")
		}
		log.V(1).Info("Get bootstrap data")
		bootstrapData, err = r.getBootstrapData(ctx, machine)
		if err != nil {
			log.Error(err, "failed to get bootstrap data")
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, WaitingForBootstrapDataReason, "Failed to get bootstrap data")
		}
	} else if scvmmMachine.Spec.CloudInit != nil {
		if scvmmMachine.Spec.CloudInit.UserData != "" {
			bootstrapData = []byte(scvmmMachine.Spec.CloudInit.UserData)
		}
		if scvmmMachine.Spec.CloudInit.MetaData != "" {
			metaData = []byte(scvmmMachine.Spec.CloudInit.MetaData)
		}
		if scvmmMachine.Spec.CloudInit.NetworkConfig != "" {
			networkConfig = []byte(scvmmMachine.Spec.CloudInit.NetworkConfig)
		}
	} else {
		// Sanity check
		return ctrl.Result{}, fmt.Errorf("No machine and no cloudinit data, CANTHAPPEN")
	}
	log.V(1).Info("Create cloudinit")
	if err := writeCloudInit(log, scvmmMachine, provider, vm.VMId, isoPath, bootstrapData, metaData, networkConfig); err != nil {
		log.Error(err, "failed to create cloud init")
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, WaitingForBootstrapDataReason, "Failed to create cloud init data")
	}
	conditions.MarkFalse(scvmmMachine, VmRunning, VmStartingReason, clusterv1.ConditionSeverityInfo, "")
	if err := patchScvmmMachine(ctx, patchHelper, scvmmMachine); err != nil {
		log.Error(err, "Failed to patch scvmmMachine", "scvmmmachine", scvmmMachine)
		return ctrl.Result{}, err
	}
	vm, err = sendWinrmCommand(log, scvmmMachine.Spec.ProviderRef, "AddIsoToVM -ID '%s' -ISOPath '%s'",
		escapeSingleQuotes(scvmmMachine.Spec.Id),
		escapeSingleQuotes(isoPath))
	if err != nil {
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, WaitingForBootstrapDataReason, "Failed to add iso to vm")
	}
	return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 10, nil, VmCreated, VmCreatingReason, "Adding ISO to VM %s", vm.Name)
}

func (r *ScvmmMachineReconciler) startVM(ctx context.Context, patchHelper *patch.Helper, cluster *clusterv1.Cluster, machine *clusterv1.Machine, provider *infrav1.ScvmmProviderSpec, scvmmMachine *infrav1.ScvmmMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	// Add adcomputer here, because we now know the vmname will not change
	// (a VM with the cloud-init iso connected is prio 1 in vmname clash resolution)
	var err error
	adspec := scvmmMachine.Spec.ActiveDirectory
	if adspec != nil {
		domaincontroller := adspec.DomainController
		if domaincontroller == "" {
			domaincontroller = provider.ADServer
		}
		_, err := sendWinrmCommand(log, scvmmMachine.Spec.ProviderRef, "CreateADComputer -Name '%s' -OUPath '%s' -DomainController '%s' -Description '%s' -MemberOf @(%s)",
			escapeSingleQuotes(scvmmMachine.Spec.VMName),
			escapeSingleQuotes(adspec.OUPath),
			escapeSingleQuotes(domaincontroller),
			escapeSingleQuotes(adspec.Description),
			escapeSingleQuotesArray(adspec.MemberOf))
		if err != nil {
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, VmFailedReason, "Failed to create AD entry")
		}
	}
	vm, err := sendWinrmCommand(log, scvmmMachine.Spec.ProviderRef, "StartVM -ID '%s'",
		escapeSingleQuotes(scvmmMachine.Spec.Id))
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "Failed to start vm")
	}
	scvmmMachine.Status.VMStatus = vm.Status
	if err := patchScvmmMachine(ctx, patchHelper, scvmmMachine); err != nil {
		log.Error(err, "Failed to patch scvmmMachine", "scvmmmachine", scvmmMachine)
		return ctrl.Result{}, err
	}
	r.recorder.Eventf(scvmmMachine, corev1.EventTypeNormal, VmStartingReason, "Powering on %s", vm.Name)
	log.V(1).Info("Requeue in 10 seconds")
	return ctrl.Result{RequeueAfter: time.Second * 10}, nil
}

func (r *ScvmmMachineReconciler) getVMInfo(ctx context.Context, patchHelper *patch.Helper, scvmmMachine *infrav1.ScvmmMachine, vm VMResult) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	if vm.IPv4Addresses != nil {
		scvmmMachine.Status.Addresses = make([]clusterv1.MachineAddress, len(vm.IPv4Addresses))
		for i := range vm.IPv4Addresses {
			scvmmMachine.Status.Addresses[i] = clusterv1.MachineAddress{
				Type:    clusterv1.MachineInternalIP,
				Address: vm.IPv4Addresses[i],
			}
		}
	}
	if vm.Hostname != "" {
		scvmmMachine.Status.Hostname = vm.Hostname
	}
	log.V(1).Info("Running, set status true")
	scvmmMachine.Status.Ready = true
	conditions.MarkTrue(scvmmMachine, VmRunning)
	if err := patchScvmmMachine(ctx, patchHelper, scvmmMachine); err != nil {
		log.Error(err, "Failed to patch scvmmMachine", "scvmmmachine", scvmmMachine)
		return ctrl.Result{}, err
	}
	if vm.IPv4Addresses == nil || vm.Hostname == "" {
		vm, err := sendWinrmCommand(log, scvmmMachine.Spec.ProviderRef, "ReadVM -ID '%s'",
			escapeSingleQuotes(scvmmMachine.Spec.Id))
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "Failed to read vm")
		}
		log.Info("Reading vm IP addresses, reschedule after 60 seconds")
		r.recorder.Eventf(scvmmMachine, corev1.EventTypeNormal, VmRunningReason, "Waiting for IP of %s", vm.Name)
		return ctrl.Result{RequeueAfter: time.Second * 60}, nil
	}
	r.recorder.Eventf(scvmmMachine, corev1.EventTypeNormal, VmRunningReason, "VM %s up and running", vm.Name)
	log.V(1).Info("Done")
	return ctrl.Result{}, nil
}

type VmDiskElem struct {
	SizeMB  int64  `json:"sizeMB"`
	VHDisk  string `json:"vhDisk,omitempty"`
	Dynamic bool   `json:"dynamic"`
}

func equalStringMap(source, target map[string]string) bool {
	for key, sourcevalue := range source {
		targetvalue, ok := target[key]
		if !ok {
			return false
		}
		if sourcevalue != targetvalue {
			return false
		}
	}
	return true
}

func makeDisksJSON(disks []infrav1.VmDisk) ([]byte, error) {
	diskarr := make([]VmDiskElem, len(disks))
	for i, d := range disks {
		if d.Size == nil {
			diskarr[i].SizeMB = 0
		} else {
			diskarr[i].SizeMB = d.Size.Value() / 1024 / 1024
		}
		diskarr[i].VHDisk = d.VHDisk
		diskarr[i].Dynamic = d.Dynamic
	}
	return json.Marshal(diskarr)
}

func (r *ScvmmMachineReconciler) generateVMName(ctx context.Context, scvmmMachine *infrav1.ScvmmMachine) (string, error) {
	// Fetch the instance
	log := ctrl.LoggerFrom(ctx)
	poolName := client.ObjectKey{Namespace: scvmmMachine.ObjectMeta.Namespace, Name: scvmmMachine.Spec.VMNameFromPool.Name}
	log.V(1).Info("Fetching scvmmnamepool", "namepool", poolName)
	scvmmNamePool := &infrav1.ScvmmNamePool{}
	if err := r.Get(ctx, poolName, scvmmNamePool); err != nil {
		return "", err
	}
	owner := &corev1.TypedLocalObjectReference{
		APIGroup: &infrav1.GroupVersion.Group,
		Kind:     "ScvmmMachine",
		Name:     scvmmMachine.ObjectMeta.Name,
	}
	seen := make(map[string]bool)
	for _, n := range scvmmNamePool.Status.VMNames {
		if ownerEquals(owner, n.Owner) {
			return n.VMName, nil
		}
		seen[n.VMName] = true
	}
	for _, nameRange := range scvmmNamePool.Spec.VMNameRanges {
		candidate := nameRange.Start
		for {
			if !seen[candidate] {
				return r.claimVMNameInPool(ctx, scvmmNamePool, candidate, owner)
			}
			candidate = incrementString(candidate)
			if nameRange.End == "" || candidate >= nameRange.End {
				break
			}
		}
	}

	return "", fmt.Errorf("All vmnames in range %s claimed", poolName.Name)
}

func ownerEquals(left *corev1.TypedLocalObjectReference, right *corev1.TypedLocalObjectReference) bool {
	if right == nil {
		return left == nil
	}
	if left == nil {
		return false
	}
	return *left.APIGroup == *right.APIGroup &&
		left.Kind == right.Kind &&
		left.Name == right.Name
}

// Increment a string as if it's a number, digits remain digits and letters remain letters.
// Basically increments the last character as either a digit or letter, and rollovers carry
func incrementString(str string) string {
	runes := []rune(str)
carry:
	for i := len(runes) - 1; i >= 0; i-- {
		r := runes[i]
		switch {
		case '0' <= r && r < '9':
			fallthrough
		case 'a' <= r && r < 'z':
			fallthrough
		case 'A' <= r && r < 'Z':
			runes[i] = r + 1
			break carry
		case r == '9':
			runes[i] = '0'
		case r == 'z':
			runes[i] = 'a'
		case r == 'Z':
			runes[i] = 'A'
		}
	}
	return string(runes)
}

func (r *ScvmmMachineReconciler) claimVMNameInPool(ctx context.Context, scvmmNamePool *infrav1.ScvmmNamePool, vmName string, owner *corev1.TypedLocalObjectReference) (string, error) {
	log := ctrl.LoggerFrom(ctx)
	scvmmNamePool.Status.VMNames = append(scvmmNamePool.Status.VMNames, infrav1.VmPoolName{
		VMName: vmName,
		Owner:  owner,
	})
	log.V(1).Info("Registering vmname in pool status", "names", scvmmNamePool.Status.VMNames, "namepool", scvmmNamePool, "name", vmName, "owner", owner)
	if err := r.Client.Status().Update(ctx, scvmmNamePool); err != nil {
		log.Error(err, "Failed to patch scvmmNamePool", "scvmmnamepool", scvmmNamePool)
		return "", err
	}
	return vmName, nil
}

func (r *ScvmmMachineReconciler) removeVMNameInPool(ctx context.Context, scvmmMachine *infrav1.ScvmmMachine) error {
	log := ctrl.LoggerFrom(ctx)
	poolName := client.ObjectKey{Namespace: scvmmMachine.ObjectMeta.Namespace, Name: scvmmMachine.Spec.VMNameFromPool.Name}
	log.V(1).Info("Fetching scvmmnamepool", "namepool", poolName)
	scvmmNamePool := &infrav1.ScvmmNamePool{}
	if err := r.Get(ctx, poolName, scvmmNamePool); err != nil {
		return client.IgnoreNotFound(err)
	}
	owner := &corev1.TypedLocalObjectReference{
		APIGroup: &infrav1.GroupVersion.Group,
		Kind:     "ScvmmMachine",
		Name:     scvmmMachine.ObjectMeta.Name,
	}
	log.V(1).Info("Removing scvmmmachine from namepool", "namepool", poolName, "owner", owner, "names", scvmmNamePool.Status.VMNames)
	var vmNames []infrav1.VmPoolName
	for _, n := range scvmmNamePool.Status.VMNames {
		if !ownerEquals(owner, n.Owner) {
			vmNames = append(vmNames, n)
		}
	}
	scvmmNamePool.Status.VMNames = vmNames
	log.V(1).Info("Patching pool status", "names", scvmmNamePool.Status.VMNames, "namepool", scvmmNamePool, "owner", owner)
	if err := r.Client.Status().Update(ctx, scvmmNamePool); err != nil {
		log.Error(err, "Failed to patch scvmmNamePool", "scvmmnamepool", scvmmNamePool)
		return err
	}
	return nil
}

func (r *ScvmmMachineReconciler) reconcileDelete(ctx context.Context, patchHelper *patch.Helper, scvmmMachine *infrav1.ScvmmMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx).WithValues("scvmmmachine", scvmmMachine.Name)
	ctx = ctrl.LoggerInto(ctx, log)

	log.V(1).Info("Do delete reconciliation")
	// If there's no finalizer do nothing
	if !controllerutil.ContainsFinalizer(scvmmMachine, MachineFinalizer) {
		return ctrl.Result{}, nil
	}
	if scvmmMachine.Spec.VMName == "" && scvmmMachine.Spec.Id == "" {
		log.V(1).Info("Machine has no vmname set, remove finalizer")
		controllerutil.RemoveFinalizer(scvmmMachine, MachineFinalizer)
		if err := patchScvmmMachine(ctx, patchHelper, scvmmMachine); err != nil {
			log.Error(err, "Failed to patch scvmmMachine", "scvmmmachine", scvmmMachine)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}
	log.V(1).Info("Set created to false, doing deletion")
	conditions.MarkFalse(scvmmMachine, VmCreated, VmDeletingReason, clusterv1.ConditionSeverityInfo, "")
	if err := patchScvmmMachine(ctx, patchHelper, scvmmMachine); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to patch ScvmmMachine")
	}

	log.Info("Doing removal of ScvmmMachine")

	vm, err := sendWinrmCommand(log, scvmmMachine.Spec.ProviderRef, "RemoveVM -ID '%s'",
		escapeSingleQuotes(scvmmMachine.Spec.Id))
	if err != nil {
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, VmFailedReason, "Failed to delete VM")
	}
	if vm.Message == "Removed" {
		adspec := scvmmMachine.Spec.ActiveDirectory
		if adspec != nil {
			r.recorder.Eventf(scvmmMachine, corev1.EventTypeNormal, VmDeletingReason, "Removing AD entry %s", scvmmMachine.Spec.VMName)
			vm, err = sendWinrmCommand(log, scvmmMachine.Spec.ProviderRef, "RemoveADComputer -Name '%s' -OUPath '%s' -DomainController '%s'",
				escapeSingleQuotes(scvmmMachine.Spec.VMName),
				escapeSingleQuotes(adspec.OUPath),
				escapeSingleQuotes(adspec.DomainController))
			if err != nil {
				return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, VmFailedReason, "Failed to remove AD entry")
			}
		}
		if scvmmMachine.Spec.VMNameFromPool != nil {
			log.V(1).Info("Remove namepool reference")
			if err := r.removeVMNameInPool(ctx, scvmmMachine); err != nil {
				return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 5, err, VmCreated, VmFailedReason, "Failed to remove namepool entry")
			}
		}
		if err := r.deleteIPAddressClaims(ctx, scvmmMachine); err != nil {
			return ctrl.Result{}, err
		}
		log.V(1).Info("Machine is removed, remove finalizer")
		controllerutil.RemoveFinalizer(scvmmMachine, MachineFinalizer)
		if perr := patchScvmmMachine(ctx, patchHelper, scvmmMachine); perr != nil {
			log.Error(perr, "Failed to patch scvmmMachine", "scvmmmachine", scvmmMachine)
			return ctrl.Result{}, perr
		}
		r.recorder.Eventf(scvmmMachine, corev1.EventTypeNormal, VmDeletingReason, "Removed vm %s", scvmmMachine.Spec.VMName)
		return ctrl.Result{}, nil
	} else {
		log.V(1).Info("Set status")
		scvmmMachine.Status.VMStatus = vm.Status
		scvmmMachine.Status.CreationTime = vm.CreationTime
		scvmmMachine.Status.ModifiedTime = vm.ModifiedTime
		log.V(1).Info("Requeue after 15 seconds")
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 15, nil, VmCreated, VmDeletingReason, "%s %s", vm.Status, scvmmMachine.Spec.VMName)
	}
}

func (r *ScvmmMachineReconciler) patchReasonCondition(ctx context.Context, patchHelper *patch.Helper, scvmmMachine *infrav1.ScvmmMachine, requeue int, err error, condition clusterv1.ConditionType, reason string, message string, messageargs ...interface{}) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	scvmmMachine.Status.Ready = false
	if err != nil {
		if message != "" {
			r.recorder.Eventf(scvmmMachine, corev1.EventTypeWarning, reason, message, messageargs...)
		} else {
			r.recorder.Eventf(scvmmMachine, corev1.EventTypeWarning, reason, "%v", err)
		}
		conditions.MarkFalse(scvmmMachine, condition, reason, clusterv1.ConditionSeverityError, message, messageargs...)
	} else {
		if message != "" {
			r.recorder.Eventf(scvmmMachine, corev1.EventTypeNormal, reason, message, messageargs...)
		}
		conditions.MarkFalse(scvmmMachine, condition, reason, clusterv1.ConditionSeverityInfo, message, messageargs...)
	}
	if perr := patchScvmmMachine(ctx, patchHelper, scvmmMachine); perr != nil {
		log.Error(perr, "Failed to patch scvmmMachine", "scvmmmachine", scvmmMachine)
	}
	if err != nil {
		scriptError := &ScriptError{}
		if !errors.As(err, &scriptError) {
			return ctrl.Result{}, errors.Wrap(err, reason)
		}
		// Requeue script errors after 60 seconds to give scvmm a breather
		requeue = 60
	}
	if requeue != 0 {
		// This is absolutely horridly stupid.  You can't multiply a Duration with an integer,
		// so you have to cast it to a "duration" which is not actually a duration as such
		// but just a scalar masquerading as a Duration to make it work.
		//
		// (If it had been done properly, you should not have been able to multiply Duration*Duration,
		//  but only Duration*int or v.v., but I guess that's too difficult gor the go devs...)
		return ctrl.Result{RequeueAfter: time.Second * time.Duration(requeue)}, nil
	}
	return ctrl.Result{}, nil
}

func patchScvmmMachine(ctx context.Context, patchHelper *patch.Helper, scvmmMachine *infrav1.ScvmmMachine) error {
	// Always update the readyCondition by summarizing the state of other conditions.
	// A step counter is added to represent progress during the provisioning process (instead we are hiding the step counter during the deletion process).
	conditions.SetSummary(scvmmMachine,
		conditions.WithConditions(
			VmCreated,
			VmRunning,
		),
		conditions.WithStepCounterIf(scvmmMachine.ObjectMeta.DeletionTimestamp.IsZero()),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	return patchHelper.Patch(
		ctx,
		scvmmMachine,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			clusterv1.ReadyCondition,
			VmCreated,
			VmRunning,
		}},
	)
}

func (r *ScvmmMachineReconciler) getBootstrapData(ctx context.Context, machine *clusterv1.Machine) ([]byte, error) {
	if machine.Spec.Bootstrap.DataSecretName == nil {
		return nil, errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	}

	s := &corev1.Secret{}
	key := client.ObjectKey{Namespace: machine.GetNamespace(), Name: *machine.Spec.Bootstrap.DataSecretName}
	if err := r.Client.Get(ctx, key, s); err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve bootstrap data secret for ScvmmMachine %s/%s", machine.GetNamespace(), machine.GetName())
	}

	value, ok := s.Data["value"]
	if !ok {
		return nil, errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return value, nil
}

func escapeSingleQuotes(str string) string {
	return strings.Replace(str, `'`, `''`, -1)
}

func escapeSingleQuotesArray(str []string) string {
	var res strings.Builder
	if len(str) == 0 {
		return ""
	}
	res.WriteString("'")
	res.WriteString(strings.Replace(str[0], `'`, `''`, -1))
	for s := range str[1:] {
		res.WriteString("','")
		res.WriteString(strings.Replace(str[s], `'`, `''`, -1))
	}
	res.WriteString("'")
	return res.String()
}

// ScvmmClusterToScvmmMachines is a handler.ToRequestsFunc to be used to enqeue
// requests for reconciliation of ScvmmMachines.
func (r *ScvmmMachineReconciler) ScvmmClusterToScvmmMachines(ctx context.Context, o client.Object) []ctrl.Request {
	result := []ctrl.Request{}
	log := ctrl.LoggerFrom(ctx)
	c, ok := o.(*infrav1.ScvmmCluster)
	if !ok {
		log.Error(errors.Errorf("expected a ScvmmCluster but got a %T", o), "failed to get ScvmmMachine for ScvmmCluster")
		return nil
	}
	log = log.WithValues("ScvmmCluster", c.Name, "Namespace", c.Namespace)

	cluster, err := util.GetOwnerCluster(ctx, r.Client, c.ObjectMeta)
	switch {
	case apierrors.IsNotFound(errors.Cause(err)) || cluster == nil:
		return result
	case err != nil:
		log.Error(err, "failed to get owning cluster")
		return result
	}

	labels := map[string]string{clusterv1.ClusterNameLabel: cluster.Name}
	machineList := &clusterv1.MachineList{}
	if err := r.Client.List(ctx, machineList, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
		log.Error(err, "failed to list ScvmmMachines")
		return nil
	}
	for _, m := range machineList.Items {
		if m.Spec.InfrastructureRef.Name == "" {
			continue
		}
		name := client.ObjectKey{Namespace: m.Namespace, Name: m.Name}
		result = append(result, ctrl.Request{NamespacedName: name})
	}

	return result
}

func (r *ScvmmMachineReconciler) ipAddressClaimToVSphereVM(ctx context.Context, o client.Object) []reconcile.Request {
	result := []ctrl.Request{}
	log := ctrl.LoggerFrom(ctx)
	c, ok := o.(*ipamv1.IPAddressClaim)
	if !ok {
		log.Error(errors.Errorf("expected a ScvmmCluster but got a %T", o), "failed to get ScvmmMachine for ScvmmCluster")
		return nil
	}

	if util.HasOwner(c.OwnerReferences, infrav1.GroupVersion.String(), []string{"ScvmmMachine"}) {
		for _, ref := range c.OwnerReferences {
			if ref.Kind == "ScvmmMachine" {
				name := client.ObjectKey{Namespace: c.Namespace, Name: ref.Name}
				result = append(result, ctrl.Request{NamespacedName: name})
			}
		}
	}
	return result
}

type ownerOrGenerationChangedPredicate struct {
	predicate.Funcs
}

func (ownerOrGenerationChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}

	return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration() ||
		!reflect.DeepEqual(e.ObjectNew.GetOwnerReferences(), e.ObjectOld.GetOwnerReferences())
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScvmmMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	extraDebug := os.Getenv("EXTRA_DEBUG")
	if extraDebug != "" {
		ExtraDebug = true
	}
	log := ctrl.LoggerFrom(ctx)
	clusterToScvmmMachines, err := util.ClusterToTypedObjectsMapper(mgr.GetClient(), &infrav1.ScvmmMachineList{}, mgr.GetScheme())
	if err != nil {
		return err
	}
	r.recorder = mgr.GetEventRecorderFor("caps-controller")
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.ScvmmMachine{}).
		WithOptions(options).
		WithEventFilter(predicate.And(
			predicates.ResourceNotPaused(log),
			ownerOrGenerationChangedPredicate{},
		)).
		Watches(
			&clusterv1.Machine{},
			handler.EnqueueRequestsFromMapFunc(util.MachineToInfrastructureMapFunc(infrav1.GroupVersion.WithKind("ScvmmMachine"))),
		).
		Watches(
			&infrav1.ScvmmCluster{},
			handler.EnqueueRequestsFromMapFunc(r.ScvmmClusterToScvmmMachines),
		).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(clusterToScvmmMachines),
			builder.WithPredicates(predicates.ClusterUnpausedAndInfrastructureReady(log)),
		).
		Watches(
			&ipamv1.IPAddressClaim{},
			handler.EnqueueRequestsFromMapFunc(r.ipAddressClaimToVSphereVM),
		).
		Complete(r)
}
