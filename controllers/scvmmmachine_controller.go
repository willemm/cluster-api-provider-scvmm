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

	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"

	infrav1 "github.com/willemm/cluster-api-provider-scvmm/api/v1alpha3"
	// "k8s.io/apimachinery/pkg/api/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"

	"encoding/base64"
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

	// Cluster-Api related statuses
	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"
	WaitingForBootstrapDataReason         = "WaitingForBootstrapData"

	VmCreatingReason = "VmCreating"
	VmStartingReason = "VmStarting"
	VmDeletingReason = "VmDeleting"
	VmFailedReason   = "VmCreationFailed"

	MachineFinalizer = "scvmmmachine.finalizers.cluster.x-k8s.io"
)

// ScvmmMachineReconciler reconciles a ScvmmMachine object
type ScvmmMachineReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// Are global variables bad? Dunno.  These hold data for the lifetime of the controller.
var (
	ScvmmHost     string
	ScvmmExecHost string
	ScvmmUsername string
	ScvmmPassword string
	ScriptDir     string
	// ReconcileScript string
	// RemoveScript    string
	FunctionScript []byte
)

// The result (passed as json) of a call to Scvmm scripts
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

// Create a winrm powershell session and seed with the function script
func CreateWinrmCmd() (*winrm.Command, error) {
	endpoint := winrm.NewEndpoint(ScvmmExecHost, 5985, false, false, nil, nil, nil, 0)
	params := winrm.DefaultParameters
	params.TransportDecorator = func() winrm.Transporter { return &winrm.ClientNTLM{} }

	client, err := winrm.NewClientWithParameters(endpoint, ScvmmUsername, ScvmmPassword, params)
	if err != nil {
		return &winrm.Command{}, err
	}
	shell, err := client.CreateShell()
	if err != nil {
		return &winrm.Command{}, err
	}
	defer shell.Close()
	cmd, err := shell.Execute("powershell.exe", "-NonInteractive", "-NoProfile", "-Command", "-")
	if err != nil {
		return &winrm.Command{}, err
	}
	cmd.Stdin.Write(FunctionScript)
	return cmd, nil
}

func GetWinrmResult(cmd *winrm.Command) (VMResult, error) {
	decoder := json.NewDecoder(cmd.Stdout)

	var res VMResult
	err := decoder.Decode(&res)
	if err != nil {
		return VMResult{}, err
	}
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
	scvmmMachine := &infrav1.ScvmmMachine{}
	if err := r.Get(ctx, req.NamespacedName, scvmmMachine); err != nil {
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
	log := r.Log.WithValues("scvmmmachine", scvmmMachine.Name)

	log.Info("Doing reconciliation of ScvmmMachine")
	cmd, err := CreateWinrmCmd()
	if err != nil {
		return ctrl.Result{}, err
	}
	defer cmd.Close()
	fmt.Fprintf(cmd.Stdin, "GetVM -VMName %q", scvmmMachine.Spec.VMName)
	vm, err := GetWinrmResult(cmd)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "Failed to get vm")
	}
	if vm.Name == "" {
		bootstrapData, err := r.getBootstrapData(ctx, machine)
		if err != nil {
			r.Log.Error(err, "failed to get bootstrap data")
			return ctrl.Result{}, err
		}
		fmt.Fprintf(cmd.Stdin, "CreateVM -Cloud %q -VMName %q -Memory %d -CPUCount %d -DiskSize %d -VMNetwork %q -BootstrapData %q",
			scvmmMachine.Spec.Cloud, scvmmMachine.Spec.VMName, (scvmmMachine.Spec.Memory.Value() / 1024 / 1024),
			scvmmMachine.Spec.CPUCount, (scvmmMachine.Spec.DiskSize.Value() / 1024 / 1024),
			scvmmMachine.Spec.VMNetwork, bootstrapData)
		vm, err = GetWinrmResult(cmd)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "Failed to create vm")
		}

		conditions.MarkFalse(scvmmMachine, VmCreated, VmCreatingReason, clusterv1.ConditionSeverityInfo, "")

		scvmmMachine.Spec.ProviderID = "scvmm://" + vm.Guid
		scvmmMachine.Status.Ready = false
		scvmmMachine.Status.VMStatus = vm.Status
		scvmmMachine.Status.BiosGuid = vm.Guid
		scvmmMachine.Status.CreationTime = vm.CreationTime
		scvmmMachine.Status.ModifiedTime = vm.ModifiedTime
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}
	conditions.MarkTrue(scvmmMachine, VmCreated)

	scvmmMachine.Spec.ProviderID = "scvmm://" + vm.Guid
	scvmmMachine.Status.Ready = (vm.Status == "Running")
	scvmmMachine.Status.VMStatus = vm.Status
	scvmmMachine.Status.BiosGuid = vm.Guid
	scvmmMachine.Status.CreationTime = vm.CreationTime
	scvmmMachine.Status.ModifiedTime = vm.ModifiedTime

	if vm.Status == "PowerOff" {
		conditions.MarkFalse(scvmmMachine, VmRunning, VmStartingReason, clusterv1.ConditionSeverityInfo, "")
		fmt.Fprintf(cmd.Stdin, "StartVM -VMName %q", scvmmMachine.Spec.VMName)
		vm, err = GetWinrmResult(cmd)
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "Failed to start vm")
		}
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	// Wait for machine to get running state
	if vm.Status != "Running" {
		return ctrl.Result{RequeueAfter: time.Second * 30}, nil
	}
	conditions.MarkTrue(scvmmMachine, VmRunning)
	return ctrl.Result{}, nil
}

func (r *ScvmmMachineReconciler) reconcileDelete(ctx context.Context, machine *clusterv1.Machine, scvmmMachine *infrav1.ScvmmMachine) (ctrl.Result, error) {
	log := r.Log.WithValues("scvmmmachine", scvmmMachine.Name)

	// If there's no finalizer do nothing
	if !controllerutil.ContainsFinalizer(scvmmMachine, MachineFinalizer) {
		return ctrl.Result{}, nil
	}
	// We are being deleted
	patchHelper, err := patch.NewHelper(scvmmMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	conditions.MarkFalse(scvmmMachine, VmCreated, VmDeletingReason, clusterv1.ConditionSeverityInfo, "")
	if err := patchScvmmMachine(ctx, patchHelper, scvmmMachine); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to patch ScvmmMachine")
	}

	log.Info("Doing removal of ScvmmMachine")
	cmd, err := CreateWinrmCmd()
	if err != nil {
		return ctrl.Result{}, err
	}
	defer cmd.Close()

	fmt.Fprintf(cmd.Stdin, "RemoveVM -VMName %q", scvmmMachine.Spec.VMName)
	vm, err := GetWinrmResult(cmd)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "Failed to remove vm")
	}
	if vm.Message == "Removed" {
		// scvmmMachine.Status.FailureReason = vm.Message
		controllerutil.RemoveFinalizer(scvmmMachine, MachineFinalizer)
		return ctrl.Result{}, nil
	} else {
		scvmmMachine.Status.VMStatus = vm.Status
		scvmmMachine.Status.CreationTime = vm.CreationTime
		scvmmMachine.Status.ModifiedTime = vm.ModifiedTime
		// if vm.Message != "" {
		//         scvmmMachine.Status.FailureReason = vm.Message
		//         scvmmMachine.Status.FailureMessage = vm.Error + vm.ScriptErrors
		// }
		return ctrl.Result{RequeueAfter: time.Second * 30}, nil
	}
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
	funcScript := initScript
	/*

		data, err = ioutil.ReadFile(ScriptDir + "/reconcile.ps1")
		if err != nil {
			return err
		}
		ReconcileScript = initScript + string(data)
		funcScript = funcScript + string(data)

		data, err = ioutil.ReadFile(ScriptDir + "/remove.ps1")
		if err != nil {
			return err
		}
		RemoveScript = initScript + string(data)
		funcScript = funcScript + string(data)
	*/
	FunctionScript = []byte(funcScript)

	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.ScvmmMachine{}).
		Complete(r)
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

func (r *ScvmmMachineReconciler) getBootstrapData(ctx context.Context, machine *clusterv1.Machine) (string, error) {
	if machine.Spec.Bootstrap.DataSecretName == nil {
		return "", errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")
	}

	s := &corev1.Secret{}
	key := client.ObjectKey{Namespace: machine.GetNamespace(), Name: *machine.Spec.Bootstrap.DataSecretName}
	if err := r.Client.Get(ctx, key, s); err != nil {
		return "", errors.Wrapf(err, "failed to retrieve bootstrap data secret for ScvmmMachine %s/%s", machine.GetNamespace(), machine.GetName())
	}

	value, ok := s.Data["value"]
	if !ok {
		return "", errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return base64.StdEncoding.EncodeToString(value), nil
}
