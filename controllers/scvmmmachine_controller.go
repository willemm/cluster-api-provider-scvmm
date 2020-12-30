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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrastructurev1alpha1 "github.com/willemm/cluster-api-provider-scvmm/api/v1alpha1"

        "io/ioutil"
        "encoding/json"
)

// ScvmmMachineReconciler reconciles a ScvmmMachine object
type ScvmmMachineReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	ScvmmHost string
	ScvmmExecHost string
	ScvmmUsername string
	ScvmmPassword string
        ScriptDir string
}

var (
        functionScript string
)

type GetVMResult struct {
  Cloud string
  Name string
  Status string
  Memory int
  CpuCount int
  VirtualNetwork string
}

func GetVMInfo(vmname string) (GetVMResult, error) {
  endpoint := winrm.NewEndpoint(ScvmmExecHost, 5985, false, false, nil, nil, nil, 0)
  params := winrm.DefaultParameters
  params.TransportDecorator = func() winrm.Transporter { return &winrm.ClientNTLM{} }

  client, err := winrm.NewClientWithParameters(endpoint, ScvmmUsername, ScvmmPassword, params)
  if err != nil {
    return nil,err
  }
  rout, rerr, rcode, err := client.RunPSWithString(functionScript+"GetVM "+vmname, "")
  if err != nil {
    return nil,err
  }
  if rcode != 0 {
    return nil,"Returncode not 0"
  }

  var res GetVMResult
  err = json.Unmarshal([]byte(rout), &res)
  if err != nil {
    return nil,err
  }
  return res,nil
}

type ReconcileVMResult struct {
  Cloud string
  Name string
  Status string
  Memory int
  CpuCount int
  VirtualNetwork string
}

func ReconcileVM(cloud string, vmname string, disksize string, vmnetwork string, memory string, cpucount string) (ReconcileVMResult, error) {
  endpoint := winrm.NewEndpoint(ScvmmExecHost, 5985, false, false, nil, nil, nil, 0)
  params := winrm.DefaultParameters
  params.TransportDecorator = func() winrm.Transporter { return &winrm.ClientNTLM{} }

  client, err := winrm.NewClientWithParameters(endpoint, ScvmmUsername, ScvmmPassword, params)
  if err != nil {
    return nil,err
  }
  rout, rerr, rcode, err := client.RunPSWithString(functionScript+
    "ReconcileVM -Cloud '"+cloud+
    "' -VMName '"+vmname+
    "' -Memory '"+memory+
    "' -CPUCount '"+cpucount+
    "' -DiskSize '"+disksize+
    "' -VMNetwork '"+vmnetwork+"'", "")
  if err != nil {
    return nil,err
  }
  if rcode != 0 {
    return nil,"Returncode not 0"
  }

  var res ReconcileVMResult
  err = json.Unmarshal([]byte(rout), &res)
  if err != nil {
    return nil,err
  }
  return res,nil
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmmachines/status,verbs=get;update;patch

func (r *ScvmmMachineReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("scvmmmachine", req.NamespacedName)

	var scMachine infrastructurev1alpha1.ScvmmMachine
	if err := r.Get(ctx, req.NamespacedName, &scMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// your logic here

	return ctrl.Result{}, nil
}

func (

func (r *ScvmmMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
        initScript := ""
        if ScvmmExecHost != "" {
          data, err := ioutil.ReadFile(ScriptDir+"/init.ps1")
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
            return "$"+key
          })
        } else {
          ScvmmExecHost = ScvmmHost
        }
        data, err = ioutil.ReadFile(ScriptDir+"/functions.ps1")
        if err != nil {
          return err
        }
        functionScript = initScript + string(data)
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.ScvmmMachine{}).
		Complete(r)
}
