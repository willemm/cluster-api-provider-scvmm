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

	"errors"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

        clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	infrav1 "github.com/willemm/cluster-api-provider-scvmm/api/v1alpha1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"encoding/json"
	"fmt"
	"github.com/masterzen/winrm"
	"io/ioutil"
	"os"
	"time"
)

const (
        // Creation started
	VmCreated clusterv1.ConditionType = "VmCreated"
        // VM running
	VmRunning clusterv1.ConditionType = "VmRunning"

	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"
	WaitingForBootstrapDataReason = "WaitingForBootstrapData"

	VmCreatingReason = "VmCreating"
	VmStartingReason = "VmStarting"
	VmDeletingReason = "VmDeleting"
	VmFailedReason = "VmCreationFailed"

	MachineFinalizer = "scvmmmachine.finalizers.cluster.x-k8s.io"
)

// ScvmmMachineReconciler reconciles a ScvmmMachine object
type ScvmmMachineReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

var (
	ScvmmHost       string
	ScvmmExecHost   string
	ScvmmUsername   string
	ScvmmPassword   string
	ScriptDir       string
	ReconcileScript string
	RemoveScript    string
)

type VMResult struct {
	Cloud          string
	Name           string
	Status         string
	Memory         int
	CpuCount       int
	VirtualNetwork string
	Guid           string
	Error          string
	ScriptErrors   string
	Message        string
	CreationTime   metav1.Time
	ModifiedTime   metav1.Time
}

func ReconcileVM(cloud string, vmname string, disksize resource.Quantity, vmnetwork string, memory resource.Quantity, cpucount int) (VMResult, error) {
	endpoint := winrm.NewEndpoint(ScvmmExecHost, 5985, false, false, nil, nil, nil, 0)
	params := winrm.DefaultParameters
	params.TransportDecorator = func() winrm.Transporter { return &winrm.ClientNTLM{} }

	client, err := winrm.NewClientWithParameters(endpoint, ScvmmUsername, ScvmmPassword, params)
	if err != nil {
		return VMResult{}, err
	}
	rout, rerr, rcode, err := client.RunPSWithString(ReconcileScript+fmt.Sprintf(
		"ReconcileVM -Cloud '%s' -VMName '%s' -Memory %d "+
			"-CPUCount %d -DiskSize %d -VMNetwork '%s'",
		cloud, vmname, (memory.Value()/1024/1024),
		cpucount, (disksize.Value()/1024/1024), vmnetwork), "")
	if err != nil {
		return VMResult{}, err
	}
	if rcode != 0 {
		return VMResult{}, fmt.Errorf("ReconcileVM script failed, returncode %d: %q", rcode, rerr)
	}

	var res VMResult
	err = json.Unmarshal([]byte(rout), &res)
	if err != nil {
		return VMResult{}, err
	}
	res.ScriptErrors = rerr
	return res, nil
}

func RemoveVM(vmname string) (VMResult, error) {
	endpoint := winrm.NewEndpoint(ScvmmExecHost, 5985, false, false, nil, nil, nil, 0)
	params := winrm.DefaultParameters
	params.TransportDecorator = func() winrm.Transporter { return &winrm.ClientNTLM{} }

	client, err := winrm.NewClientWithParameters(endpoint, ScvmmUsername, ScvmmPassword, params)
	if err != nil {
		return VMResult{}, err
	}
	rout, rerr, rcode, err := client.RunPSWithString(RemoveScript+
		"RemoveVM -VMName '"+vmname+"'", "")
	if err != nil {
		return VMResult{}, err
	}
	if rcode != 0 {
		return VMResult{}, fmt.Errorf("RemoveVM script failed, returncode %d: %q", rcode, rerr)
	}

	var res VMResult
	err = json.Unmarshal([]byte(rout), &res)
	if err != nil {
		return VMResult{}, err
	}
	res.ScriptErrors = rerr
	return res, nil
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;machines,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

func (r *ScvmmMachineReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, retErr error) {
	ctx := context.Background()
	log := r.Log.WithValues("scvmmmachine", req.NamespacedName)

        // Fetch the instance
	var scvmmMachine infrav1.ScvmmMachine
	if err := r.Get(ctx, req.NamespacedName, &scvmmMachine); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, scvmmMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("Waiting for Machine Controller to set OwnerRef on ScvmmMachine")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("machine", machine.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Info("ScvmmMachine owner Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info(fmt.Sprintf("Please associate this machine with a cluster using the label %s: <name of cluster>", clusterv1.ClusterLabelName))
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	// Fetch the Scvmm Cluster.
	scvmmCluster := &infrav1.ScvmmCluster{}
	scvmmClusterName := client.ObjectKey{
		Namespace: scvmmMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, scvmmClusterName, scvmmCluster); err != nil {
		log.Info("ScvmmCluster is not available yet")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("scvmm-cluster", scvmmCluster.Name)

	patchHelper, err := patch.NewHelper(scvmmMachine, r)
	if err != nil {
		return ctrl.Result{}, err
	}
	defer func() {
		if err := patchScvmmMachine(ctx, patchHelper, scvmmMachine); err != nil {
			log.Error(err, "failed to patch ScvmmMachine")
			if retErr == nil {
				retErr = err
			}
		}
	}()

        // Handle deleted machines
        // NB: The reference implementation handles deletion at the end of this function, but that seems wrogn
        //     because AIUI that could lead to trying to add a finalizer to a resource being deleted
	if !scvmmMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machine, scvmmMachine)
	}

	// Add finalizer.  Apparently we should return here to avoid a race condition
        // (Presumably the change/patch will trigger another reconciliation so it continues)
	if !controllerutil.ContainsFinalizer(scvmmMachine, MachineFinalizer) {
		controllerutil.AddFinalizer(scvmmMachine, MachineFinalizer)
		return ctrl.Result{}, nil
	}

	// Check if the infrastructure is ready, otherwise return and wait for the cluster object to be updated
	if !cluster.Status.InfrastructureReady {
		log.Info("Waiting for ScvmmCluster Controller to create cluster infrastructure")
		conditions.MarkFalse(scvmmMachine, VmCreated, WaitingForClusterInfrastructureReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, cluster, machine, scvmmMachine)
}

func (r *ScvmmMachineReconciler) reconcileNormal(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine, scvmmMachine *infrav1.ScvmmMachine) (res ctrl.Result, retErr error) {
	log := r.Log.WithValues("scvmmmachine", req.NamespacedName)

        vm, err := ReconcileVM(scvmmMachine.Spec.Cloud, scvmmMachine.Spec.VMName, scvmmMachine.Spec.DiskSize,
                scvmmMachine.Spec.VMNetwork, scvmmMachine.Spec.Memory, scvmmMachine.Spec.CPUCount)
        if err != nil {
                return ctrl.Result{}, err
        }

        scvmmMachine.Spec.ProviderID = "scvmm://" + vm.Guid
        scvmmMachine.Status.Ready = (vm.Status == "Running")
        scvmmMachine.Status.VMStatus = vm.Status
        scvmmMachine.Status.BiosGuid = vm.Guid
        scvmmMachine.Status.CreationTime = vm.CreationTime
        scvmmMachine.Status.ModifiedTime = vm.ModifiedTime
        if vm.Message != "" {
                scvmmMachine.Status.FailureReason = vm.Message
                scvmmMachine.Status.FailureMessage = vm.Error + vm.ScriptErrors
        }
        // Wait for machine to get running state
        if vm.Status != "Running" {
                return ctrl.Result{RequeueAfter: time.Second * 30}, nil
        }
        return ctrl.Result{}, nil
}

func (r *ScvmmMachineReconciler) reconcileDelete(ctx context.Context, machine *clusterv1.Machine, scvmmMachine *infrav1.ScvmmMachine) (ctrl.Result, error) {
        // If there's no finalizer do nothing
	if !controllerutil.ContainsFinalizer(scvmmMachine, MachineFinalizer) {
                return ctrl.Result{}, nil
        }
        // We are being deleted
        // HERE.I.AM
	patchHelper, err := patch.NewHelper(scvmmMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	conditions.MarkFalse(scvmmMachine, VmCreated, VmDeletingReason, clusterv1.ConditionSeverityInfo, "")
	if err := patchScvmmMachine(ctx, patchHelper, scvmmMachine); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to patch ScvmmMachine")
	}

        log.Info("Doing removal of ScvmmMachine")
        vm, err := RemoveVM(scvmmMachine.Spec.VMName)
        if err != nil {
                log.Error(err, "Removal failed")
                return ctrl.Result{}, err
        }
        if vm.Message == "Removed" {
                scvmmMachine.Status.FailureReason = vm.Message
                scvmmMachine.ObjectMeta.Finalizers = removeString(scvmmMachine.ObjectMeta.Finalizers, finalizerName)
                if err := r.Update(ctx, &scvmmMachine); err != nil {
                        log.Error(err, "Failed to remove finalizer")
                        return ctrl.Result{}, err
                }
                return ctrl.Result{}, nil
        } else {
                scvmmMachine.Status.VMStatus = vm.Status
                scvmmMachine.Status.CreationTime = vm.CreationTime
                scvmmMachine.Status.ModifiedTime = vm.ModifiedTime
                if vm.Message != "" {
                        scvmmMachine.Status.FailureReason = vm.Message
                        scvmmMachine.Status.FailureMessage = vm.Error + vm.ScriptErrors
                }
                helper, err := patch.NewHelper(&scvmmMachine, r.Client)
                if err != nil {
                        return ctrl.Result{}, err
                }
                if err := helper.Patch(ctx, &scvmmMachine); err != nil {
                        log.Error(err, "Failed to update status")
                        return ctrl.Result{}, err
                }
                return ctrl.Result{RequeueAfter: time.Second * 30}, nil
        }
	return ctrl.Result{RequeueAfter: time.Minute * 10}, nil
}

func (r *ScvmmMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ScvmmHost = os.Getenv("SCVMM_HOST")
	if ScvmmHost == "" {
		return fmt.Errorf("missing required env SCVMM_HOST")
	}
	ScvmmExecHost = os.Getenv("SCVMM_EXECHOST")
	ScriptDir = os.Getenv("SCRIPT_DIR")

	ScvmmUsername = os.Getenv("SCVMM_USERNAME")
	if ScvmmUsername == "" {
		return fmt.Errorf("missing required env SCVMM_USERNAME")
	}
	ScvmmPassword = os.Getenv("SCVMM_PASSWORD")
	if ScvmmPassword == "" {
		return fmt.Errorf("missing required env SCVMM_PASSWORD")
	}

	initScript := ""
	if ScvmmExecHost != "" {
		data, err := ioutil.ReadFile(ScriptDir + "/init.ps1")
		if err != nil {
			return err
		}
		initScript = os.Expand(string(data), func(key string) string {
			switch key {
			case "SCVMM_USERNAME":
				return ScvmmUsername
			case "SCVMM_PASSWORD":
				return ScvmmPassword
			case "SCVMM_HOST":
				return ScvmmHost
			}
			return "$" + key
		})
	} else {
		ScvmmExecHost = ScvmmHost
	}
	data, err := ioutil.ReadFile(ScriptDir + "/functions.ps1")
	if err != nil {
		return err
	}
	initScript = initScript + string(data)

	data, err = ioutil.ReadFile(ScriptDir + "/reconcile.ps1")
	if err != nil {
		return err
	}
	ReconcileScript = initScript + string(data)

	data, err = ioutil.ReadFile(ScriptDir + "/remove.ps1")
	if err != nil {
		return err
	}
	RemoveScript = initScript + string(data)
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.ScvmmMachine{}).
		Complete(r)
}

// Helper functions to check and remove string from a slice of strings.
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
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
