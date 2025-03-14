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
	ipamv1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
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

	"github.com/go-logr/logr"
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

	cloudInitDeviceTypeFunctions = map[string]string{
		"":       "AddISOToVM",
		"dvd":    "AddISOToVM",
		"floppy": "AddFloppyToVM",
		"scsi":   "AddVHDToVM",
		"ide":    "AddVHDToVM",
	}
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

	if !scvmmMachine.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, patchHelper, scvmmMachine)
	}

	var cluster *clusterv1.Cluster
	var machine *clusterv1.Machine
	// If the user provides a bootstrap section in the scvmmmachine, assume it's a standalone machine
	// Otherwise get the owning machine, cluster, etc.
	if scvmmMachine.Spec.Bootstrap == nil {
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
		if scvmmMachine.Spec.ProviderRef == nil || scvmmMachine.Spec.Cloud == "" || scvmmMachine.Spec.HostGroup == "" {
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
			// Get cloud and hostgroup from failureDomains if needed
			if scvmmMachine.Spec.Cloud == "" || scvmmMachine.Spec.HostGroup == "" {
				if machine.Spec.FailureDomain == nil {
					return ctrl.Result{}, fmt.Errorf("missing failureDomain")
				}
				fd, ok := scvmmCluster.Spec.FailureDomains[*machine.Spec.FailureDomain]
				if !ok {
					return ctrl.Result{}, fmt.Errorf("unknown failureDomain %s", *machine.Spec.FailureDomain)
				}
				scvmmMachine.Spec.Cloud = fd.Cloud
				scvmmMachine.Spec.HostGroup = fd.HostGroup
				if scvmmMachine.Spec.Networking == nil {
					scvmmMachine.Spec.Networking = fd.Networking
				}
			}
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

	if vm.Status == "UnderCreation" || vm.Status == "UnderUpdate" {
		log.V(1).Info("Creating, Requeue in 15 seconds")
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 15, nil, VmCreated, VmCreatingReason, "")
	}
	log.V(1).Info("Machine is there, fill in status")
	conditions.MarkTrue(scvmmMachine, VmCreated)
	if r.vmCanAddPersistentDisks(scvmmMachine, vm) {
	}
	if vm.Status == "PowerOff" {
		if err := r.addVMSpec(ctx, patchHelper, scvmmMachine); err != nil {
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, VmFailedReason, "Failed calling add spec function")
		}
		if vmNeedsExpandDisks(scvmmMachine, vm) {
			return r.expandDisks(ctx, patchHelper, scvmmMachine)
		}
		if !hasAllIPAddresses(scvmmMachine.Spec.Networking) {
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, nil, VmCreated, WaitingForIPAddressReason, "")
		}

		log.V(1).Info("Get provider")
		provider, err := getProvider(scvmmMachine.Spec.ProviderRef)
		if err != nil {
			log.Error(err, "Failed to get provider")
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, ProviderNotAvailableReason, "")
		}

		ciPath, err := cloudInitPath(ctx, provider, scvmmMachine)
		if err != nil {
			log.Error(err, "Failed to get provider")
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, ProviderNotAvailableReason, "")
		}
		if vmNeedsCloudInit(ciPath, scvmmMachine, vm) {
			return r.addCloudInitToVM(ctx, patchHelper, cluster, machine, provider, scvmmMachine, vm, ciPath)
		}
		if (scvmmMachine.Spec.Tag != "" && vm.Tag != scvmmMachine.Spec.Tag) || !equalStringMap(scvmmMachine.Spec.CustomProperty, vm.CustomProperty) {
			return r.setVMProperties(ctx, patchHelper, scvmmMachine)
		}
		return r.startVM(ctx, patchHelper, provider, scvmmMachine)
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

func (r *ScvmmMachineReconciler) getVMIDsByName(ctx context.Context, providerRef *infrav1.ScvmmProviderReference, VMName string) ([]string, error) {
	log := ctrl.LoggerFrom(ctx)
	result, err := sendWinrmCommandWithErrorResult[VMIDsResult](log, providerRef, "GetVMIDsByName -VMName '%s'", escapeSingleQuotes(VMName))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get VMIDs by name %s", VMName)
	}
	return result.VMIDs, nil
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
			// Sanity check. Under no circumstances should we should use a VMName that already exists. generateVMName should have avoided it, but make sure.
			vmIdsByName, err := r.getVMIDsByName(ctx, spec.ProviderRef, vmName)
			if err != nil {
				return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, VmFailedReason, "Failed to check if VMName already exists in SCVMM")
			}
			if len(vmIdsByName) > 0 {
				return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, nil, VmCreated, VmFailedReason, fmt.Sprintf("VMName already exists in SCVMM, cannot use it; VMName: '%s', VMIDs: '%s'", vmName, strings.Join(vmIdsByName, ", ")))
			}
			scvmmMachine.Spec.VMName = vmName
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, nil, VmCreated, VmCreatingReason, "Set VMName %s", vmName)
		} else {
			vmName = scvmmMachine.Name
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
	fcjson, err := json.Marshal(spec.FibreChannel)
	if err != nil {
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, errors.Wrap(err, "Failed to serialize fibrechannel"), VmCreated, VmFailedReason, "Failed to create vm")
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
	vm, err := sendWinrmCommand(log, spec.ProviderRef, "CreateVM -Cloud '%s' -HostGroup '%s' -VMName '%s' -VMTemplate '%s' -Memory %d -MemoryMin %d -MemoryMax %d -MemoryBuffer %d -CPUCount %d -Disks '%s' -NetworkDevices '%s' -FibreChannel '%s' -HardwareProfile '%s' -OperatingSystem '%s' -AvailabilitySet '%s' -VMOptions '%s'",
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
		escapeSingleQuotes(string(fcjson)),
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

func vmNeedsCloudInit(ciPath string, scvmmMachine *infrav1.ScvmmMachine, vm VMResult) bool {
	// If there is an empty bootstrap section, don't add an iso
	if scvmmMachine.Spec.Bootstrap != nil {
		if scvmmMachine.Spec.Bootstrap.DataSecretName == nil {
			return false
		}
	}
	ciBase := baseName(ciPath)
	for _, iso := range vm.ISOs {
		if baseName(iso.SharePath) == ciBase {
			return false
		}
	}
	for _, vhd := range vm.VirtualDisks {
		if baseName(vhd.SharePath) == ciBase {
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

func vmDiskByLun(vm *VMResult, lun int) *VMResultDisk {
	for d := range vm.VirtualDisks {
		if d.LUN == lun {
			return d
		}
	}
	return nil
}

// Check if there is a persistent disk that hasn't been assigned yet
// and assign if possible
func (r *ScvmmMachineReconciler) vmCanAddPersistentDisks(scvmmMachine *infrav1.ScvmmMachine, vm *VMResult) (bool, error) {
	for i, d := range scvmmMachine.Spec.Disks {
		if d.PersistentDisk != nil {
			vd := vmDiskByLun(vm, i)
			if d.PersistentDisk.Disk == nil {
				// If there are free slots, grab the first free slot and claim it,
				// setting the reference in the disks array, and return true
			}
		}
	}
	return false
}

func vmNeedsExpandDisks(scvmmMachine *infrav1.ScvmmMachine, vm *VMResult) bool {
	for i, d := range scvmmMachine.Spec.Disks {
		// For rounding errors
		if d.Size != nil && vm.VirtualDisks[i].MaximumSize < (d.Size.Value()-1024*1024) {
			return true
		}
		if d.IOPSMaximum != nil && d.IOPSMaximum.Value() != vm.VirtualDisks[i].IOPSMaximum {
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

func (r *ScvmmMachineReconciler) addCloudInitToVM(ctx context.Context, patchHelper *patch.Helper, cluster *clusterv1.Cluster, machine *clusterv1.Machine, provider *infrav1.ScvmmProviderSpec, scvmmMachine *infrav1.ScvmmMachine, vm VMResult, ciPath string) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	var bootstrapData, metaData, networkConfig []byte
	var err error
	deviceFunction, ok := cloudInitDeviceTypeFunctions[provider.CloudInit.DeviceType]
	if !ok {
		log.Error(err, "Unknown devicetype "+provider.CloudInit.DeviceType)
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, WaitingForBootstrapDataReason, "Unknown devicetype "+provider.CloudInit.DeviceType)
	}
	dataSecretName := ""
	dataSecretNamespace := ""
	if scvmmMachine.Spec.Bootstrap != nil {
		if scvmmMachine.Spec.Bootstrap.DataSecretName == nil {
			// Sanity check
			return ctrl.Result{}, fmt.Errorf("No bootstrap secret, CANTHAPPEN")
		}
		dataSecretName = *scvmmMachine.Spec.Bootstrap.DataSecretName
		dataSecretNamespace = scvmmMachine.Namespace
	} else if machine != nil {
		if machine.Spec.Bootstrap.DataSecretName == nil {
			if !util.IsControlPlaneMachine(machine) && !conditions.IsTrue(cluster, clusterv1.ControlPlaneInitializedCondition) {
				log.Info("Waiting for the control plane to be initialized")
				return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, nil, VmCreated, WaitingForControlPlaneAvailableReason, "")
			}
			log.Info("Waiting for the Bootstrap provider controller to set bootstrap data")
			return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, nil, VmCreated, WaitingForBootstrapDataReason, "")
		}
		dataSecretName = *machine.Spec.Bootstrap.DataSecretName
		dataSecretNamespace = machine.Namespace
	} else {
		// Sanity check
		return ctrl.Result{}, fmt.Errorf("No machine and no bootstrap data, CANTHAPPEN")
	}

	log.V(1).Info("Get bootstrap data")
	bootstrapData, err = r.getBootstrapData(ctx, dataSecretName, dataSecretNamespace)
	if err != nil {
		log.Error(err, "failed to get bootstrap data")
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, WaitingForBootstrapDataReason, "Failed to get bootstrap data")
	}
	log.V(1).Info("Create cloudinit")
	if err := writeCloudInit(log, scvmmMachine, provider, vm.VMId, ciPath, bootstrapData, metaData, networkConfig); err != nil {
		log.Error(err, "failed to create cloud init")
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, WaitingForBootstrapDataReason, "Failed to create cloud init data")
	}
	conditions.MarkFalse(scvmmMachine, VmRunning, VmStartingReason, clusterv1.ConditionSeverityInfo, "")
	if err := patchScvmmMachine(ctx, patchHelper, scvmmMachine); err != nil {
		log.Error(err, "Failed to patch scvmmMachine", "scvmmmachine", scvmmMachine)
		return ctrl.Result{}, err
	}

	vm, err = sendWinrmCommand(log, scvmmMachine.Spec.ProviderRef, deviceFunction+" -ID '%s' -CIPath '%s' -DeviceType '%s'",
		escapeSingleQuotes(scvmmMachine.Spec.Id),
		escapeSingleQuotes(ciPath),
		escapeSingleQuotes(provider.CloudInit.DeviceType))
	if err != nil {
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, err, VmCreated, WaitingForBootstrapDataReason, "Failed to add iso to vm")
	}
	return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 10, nil, VmCreated, VmCreatingReason, "Adding cloud-init to VM %s", vm.Name)
}

func (r *ScvmmMachineReconciler) startVM(ctx context.Context, patchHelper *patch.Helper, provider *infrav1.ScvmmProviderSpec, scvmmMachine *infrav1.ScvmmMachine) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	// Before adding to AD and starting the VM, make sure there's only one VM with this name.
	// This was already checked in namepool and before createVM, but since one might have multiple controllers, check again.
	vmIdsByName, errDupCheck := r.getVMIDsByName(ctx, scvmmMachine.Spec.ProviderRef, scvmmMachine.Spec.VMName)
	if errDupCheck != nil {
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, errDupCheck, VmCreated, VmFailedReason, "Failed to check if VMName already exists in SCVMM before AD+start")
	}
	if len(vmIdsByName) > 1 {
		return r.patchReasonCondition(ctx, patchHelper, scvmmMachine, 0, nil, VmCreated, VmFailedReason, fmt.Sprintf("VMName already exists in SCVMM before AD+start, cannot use it; VMName: '%s', VMIDs: '%s', expected ID: '%s'", scvmmMachine.Spec.VMName, strings.Join(vmIdsByName, ", "), scvmmMachine.Spec.Id))
	}
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
	SizeMB           int64  `json:"sizeMB"`
	VHDisk           string `json:"vhDisk,omitempty"`
	Dynamic          bool   `json:"dynamic"`
	VolumeType       string `json:"volumeType,omitempty"`
	StorageQoSPolicy string `json:"storageQoSPolicy,omitempty"`
	IOPSMaximum      int64  `json:"iopsMaximum"`
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
		diskarr[i].VolumeType = d.VolumeType
		diskarr[i].StorageQoSPolicy = d.StorageQoSPolicy
		if d.IOPSMaximum == nil {
			diskarr[i].IOPSMaximum = 0
		} else {
			diskarr[i].IOPSMaximum = d.IOPSMaximum.Value()
		}
	}
	return json.Marshal(diskarr)
}

func (r *ScvmmMachineReconciler) generateVMName(ctx context.Context, scvmmMachine *infrav1.ScvmmMachine) (string, error) {
	// Fetch the instance
	log := ctrl.LoggerFrom(ctx)
	poolName := client.ObjectKey{Namespace: scvmmMachine.Namespace, Name: scvmmMachine.Spec.VMNameFromPool.Name}
	log.V(1).Info("Fetching scvmmnamepool", "namepool", poolName)
	scvmmNamePool := &infrav1.ScvmmNamePool{}
	if err := r.Get(ctx, poolName, scvmmNamePool); err != nil {
		return "", err
	}
	log.V(1).Info("Retrieving vmnames from ScvmmMachines", "namespace", scvmmNamePool.Namespace)
	if scvmmNamePool.Status.VMNameOwners == nil {
		scvmmNamePool.Status.VMNameOwners = make(map[string]string)
	}
	owners := scvmmNamePool.Status.VMNameOwners
	for vmName, owner := range owners {
		if owner == scvmmMachine.Name {
			return vmName, nil
		}
	}
	vmList := &infrav1.ScvmmMachineList{}
	if err := r.List(ctx, vmList, client.InNamespace(scvmmNamePool.Namespace)); err != nil {
		return "", err
	}
	for _, vm := range vmList.Items {
		if vm.Spec.VMName != "" && owners[vm.Spec.VMName] == "" {
			for _, nameRange := range scvmmNamePool.Spec.VMNameRanges {
				if (nameRange.End == "" && nameRange.Start == vm.Spec.VMName) ||
					(nameRange.Start <= vm.Spec.VMName && nameRange.End >= vm.Spec.VMName) {
					owners[vm.Spec.VMName] = vm.Name
					break
				}
			}
		}
	}
	vmName := ""
	counts := &infrav1.ScvmmPoolCounts{}
	for _, nameRange := range scvmmNamePool.Spec.VMNameRanges {
		c := nameRange.Start
		for {
			candidate := c + nameRange.Postfix
			if vmName == "" && owners[candidate] == "" {
				// Found an unused name in the pool; make sure SCVMM doesn't already have a VM with that name.
				vmIdsByName, err := r.getVMIDsByName(ctx, scvmmMachine.Spec.ProviderRef, candidate)
				if err != nil {
					return "", err
				}
				if len(vmIdsByName) != 0 {
					log.V(1).Info("VMName already exists in SCVMM, skipping", "name", candidate, "ids", vmIdsByName)
				} else {
					vmName = candidate
					owners[candidate] = scvmmMachine.Name
					log.V(1).Info("Registering vmname in pool status", "vmnameOwners", scvmmNamePool.Status.VMNameOwners, "namepool", scvmmNamePool, "name", vmName, "owner", scvmmMachine.Name)
				}
			}
			counts.Total++
			if owners[candidate] == "" {
				counts.Free++
			} else {
				counts.Used++
			}
			c = incrementString(c)
			if nameRange.End == "" || c > nameRange.End {
				break
			}
		}
	}

	scvmmNamePool.Status.Counts = counts
	if err := r.Client.Status().Update(ctx, scvmmNamePool); err != nil {
		log.Error(err, "Failed to patch scvmmNamePool", "scvmmnamepool", scvmmNamePool)
		return "", err
	}
	if vmName == "" {
		return "", fmt.Errorf("All vmnames in range %s claimed", poolName.Name)
	}
	return vmName, nil
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

func (r *ScvmmMachineReconciler) removeVMNameInPool(ctx context.Context, scvmmMachine *infrav1.ScvmmMachine) error {
	log := ctrl.LoggerFrom(ctx)
	poolName := client.ObjectKey{Namespace: scvmmMachine.Namespace, Name: scvmmMachine.Spec.VMNameFromPool.Name}
	log.V(1).Info("Fetching scvmmnamepool", "namepool", poolName)
	scvmmNamePool := &infrav1.ScvmmNamePool{}
	if err := r.Get(ctx, poolName, scvmmNamePool); err != nil {
		return client.IgnoreNotFound(err)
	}
	vmList := &infrav1.ScvmmMachineList{}
	if err := r.List(ctx, vmList, client.InNamespace(scvmmNamePool.Namespace)); err != nil {
		return err
	}
	// Also remove status vmname entries whose owner is no longer there
	seen := make(map[string]bool)
	for _, vm := range vmList.Items {
		seen[vm.Name] = true
	}
	log.V(1).Info("Removing scvmmmachine from namepool", "namepool", poolName, "owner", scvmmMachine.Name, "names", scvmmNamePool.Status.VMNameOwners)
	vmNameOwners := make(map[string]string)
	for vmName, owner := range scvmmNamePool.Status.VMNameOwners {
		if seen[owner] {
			if owner == scvmmMachine.Name {
				if scvmmNamePool.Status.Counts != nil {
					scvmmNamePool.Status.Counts.Used--
					scvmmNamePool.Status.Counts.Free++
				}
			} else {
				vmNameOwners[vmName] = owner
			}
		}
	}
	scvmmNamePool.Status.VMNameOwners = vmNameOwners
	log.V(1).Info("Patching pool status", "names", scvmmNamePool.Status.VMNameOwners, "namepool", scvmmNamePool)
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
		conditions.WithStepCounterIf(scvmmMachine.DeletionTimestamp.IsZero()),
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

func (r *ScvmmMachineReconciler) getBootstrapData(ctx context.Context, name, namespace string) ([]byte, error) {
	s := &corev1.Secret{}
	key := client.ObjectKey{Namespace: namespace, Name: name}
	if err := r.Client.Get(ctx, key, s); err != nil {
		return nil, errors.Wrapf(err, "failed to retrieve bootstrap data secret for ScvmmMachine %s/%s", namespace, name)
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

type ownerOrGenerationChangedPredicate struct {
	predicate.Funcs
	log logr.Logger
}

func (p ownerOrGenerationChangedPredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return true
	}
	if e.ObjectNew == nil {
		return true
	}
	if _, ok := e.ObjectNew.(*infrav1.ScvmmMachine); ok {
		return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration() ||
			!reflect.DeepEqual(e.ObjectNew.GetOwnerReferences(), e.ObjectOld.GetOwnerReferences())
	}
	return true

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
			ownerOrGenerationChangedPredicate{log: log},
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
		Owns(&ipamv1.IPAddressClaim{}, builder.MatchEveryOwner).
		Complete(r)
}
