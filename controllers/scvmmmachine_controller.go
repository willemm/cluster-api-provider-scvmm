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
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrastructurev1alpha1 "github.com/willemm/cluster-api-provider-scvmm/api/v1alpha1"

	"encoding/json"
	"fmt"
	"github.com/masterzen/winrm"
	"io/ioutil"
	"os"
	"time"
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
	CreationTime   string
	ModifiedTime   string
}

func ReconcileVM(cloud string, vmname string, disksize string, vmnetwork string, memory string, cpucount string) (VMResult, error) {
	endpoint := winrm.NewEndpoint(ScvmmExecHost, 5985, false, false, nil, nil, nil, 0)
	params := winrm.DefaultParameters
	params.TransportDecorator = func() winrm.Transporter { return &winrm.ClientNTLM{} }

	client, err := winrm.NewClientWithParameters(endpoint, ScvmmUsername, ScvmmPassword, params)
	if err != nil {
		return VMResult{}, err
	}
	rout, rerr, rcode, err := client.RunPSWithString(ReconcileScript+
		"ReconcileVM -Cloud '"+cloud+
		"' -VMName '"+vmname+
		"' -Memory '"+memory+
		"' -CPUCount '"+cpucount+
		"' -DiskSize '"+disksize+
		"' -VMNetwork '"+vmnetwork+"'", "")
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

func (r *ScvmmMachineReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("scvmmmachine", req.NamespacedName)

	var scMachine infrastructurev1alpha1.ScvmmMachine
	if err := r.Get(ctx, req.NamespacedName, &scMachine); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	finalizerName := "scvmmmachines.finalizers.cluster.x-k8s.io"
	if scMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(scMachine.ObjectMeta.Finalizers, finalizerName) {
			log.Info("Adding finalizer to ScvmmMachine")
			scMachine.ObjectMeta.Finalizers = append(scMachine.ObjectMeta.Finalizers, finalizerName)
			if err := r.Update(ctx, &scMachine); err != nil {
				log.Error(err, "Failed to add finalizer")
				return ctrl.Result{}, err
			}
		}
		vm, err := ReconcileVM(scMachine.Spec.Cloud, scMachine.Spec.VMName, scMachine.Spec.DiskSize,
			scMachine.Spec.VMNetwork, scMachine.Spec.Memory, scMachine.Spec.CPUCount)
		if err != nil {
			return ctrl.Result{}, err
		}

		helper, err := patch.NewHelper(&scMachine, r.Client)
		if err != nil {
			return ctrl.Result{}, err
		}
		providerID := "scvmm://" + vm.Guid
		scMachine.Spec.ProviderID = providerID
		scMachine.Status.Ready = (vm.Status == "Running")
		scMachine.Status.VMStatus = vm.Status
		scMachine.Status.BiosGuid = vm.Guid
		scMachine.Status.CreationTime = vm.CreationTime
		scMachine.Status.ModifiedTime = vm.ModifiedTime
		if vm.Message != "" {
			scMachine.Status.FailureReason = vm.Message
			scMachine.Status.FailureMessage = vm.Error + vm.ScriptErrors
		}
		if err := helper.Patch(ctx, &scMachine); err != nil {
			return ctrl.Result{}, err
		}
		if vm.Status != "Running" {
			return ctrl.Result{RequeueAfter: time.Second * 30}, nil
		}
	} else {
		// We are being deleted
		if containsString(scMachine.ObjectMeta.Finalizers, finalizerName) {
			log.Info("Doing removal of ScvmmMachine")
			scMachine.Status.Ready = false
			vm, err := RemoveVM(scMachine.Spec.VMName)
			if err != nil {
				log.Error(err, "Removal failed")
				return ctrl.Result{}, err
			}
			if vm.Message == "Removed" {
				scMachine.Status.FailureReason = vm.Message
				scMachine.ObjectMeta.Finalizers = removeString(scMachine.ObjectMeta.Finalizers, finalizerName)
				if err := r.Update(ctx, &scMachine); err != nil {
					log.Error(err, "Failed to remove finalizer")
					return ctrl.Result{}, err
				}
				return ctrl.Result{}, nil
			} else {
				scMachine.Status.VMStatus = vm.Status
				scMachine.Status.CreationTime = vm.CreationTime
				scMachine.Status.ModifiedTime = vm.ModifiedTime
				if vm.Message != "" {
					scMachine.Status.FailureReason = vm.Message
					scMachine.Status.FailureMessage = vm.Error + vm.ScriptErrors
				}
				helper, err := patch.NewHelper(&scMachine, r.Client)
				if err != nil {
					return ctrl.Result{}, err
				}
				if err := helper.Patch(ctx, &scMachine); err != nil {
					log.Error(err, "Failed to update status")
					return ctrl.Result{}, err
				}
				return ctrl.Result{RequeueAfter: time.Second * 30}, nil
			}
		}
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
		For(&infrastructurev1alpha1.ScvmmMachine{}).
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
