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

	// "encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/masterzen/winrm"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2"
	"net"
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
	MissingClusterReason                  = "MissingCluster"

	VmCreatingReason = "VmCreating"
	VmStartingReason = "VmStarting"
	VmDeletingReason = "VmDeleting"
	VmFailedReason   = "VmFailed"

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
	// Host where the scvmm lives
	ScvmmHost string
	// Remote (windows) host that we run the powershell scripts on (only if different from scvmm host)
	// +optional
	ScvmmExecHost string
	// Username to run the scripts under
	ScvmmUsername string
	// Password for running the scripts
	ScvmmPassword string
	// Location of Scvmm Library Share
	ScvmmLibraryShare string
	// Location of the powershell script files
	ScriptDir string
	// Holds the powershell script-functions (will be executed at the start of each remote shell)
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
	IPv4Addresses  []string
	Guid           string
	Error          string
	ScriptErrors   string
	Message        string
	CreationTime   metav1.Time
	ModifiedTime   metav1.Time
}

// Create a winrm powershell session and seed with the function script
func createWinrmCmd() (*winrm.Command, error) {
	endpoint := winrm.NewEndpoint(ScvmmExecHost, 5985, false, false, nil, nil, nil, 0)
	params := winrm.DefaultParameters
	params.TransportDecorator = func() winrm.Transporter { return &winrm.ClientNTLM{} }

	client, err := winrm.NewClientWithParameters(endpoint, ScvmmUsername, ScvmmPassword, params)
	if err != nil {
		return &winrm.Command{}, errors.Wrap(err, "Creating winrm client")
	}
	shell, err := client.CreateShell()
	if err != nil {
		return &winrm.Command{}, errors.Wrap(err, "Creating winrm shell")
	}
	cmd, err := shell.Execute("powershell.exe", "-NonInteractive", "-NoProfile", "-Command", "-")
	if err != nil {
		return &winrm.Command{}, errors.Wrap(err, "Creating winrm powershell")
	}
	_, err = cmd.Stdin.Write(FunctionScript)
	if err != nil {
		return &winrm.Command{}, errors.Wrap(err, "Sending powershell functions")
	}
	return cmd, nil
}

func getWinrmResult(cmd *winrm.Command) (VMResult, error) {
	decoder := json.NewDecoder(cmd.Stdout)

	var res VMResult
	err := decoder.Decode(&res)
	if err != nil {
		outb := make([]byte, 1024)
		n, _ := cmd.Stdout.Read(outb)
		return VMResult{}, errors.Wrapf(err, "Decoding script result %q", string(outb[:n]))
	}
	return res, nil
}

func sendWinrmCommand(log logr.Logger, cmd *winrm.Command, command string, args ...interface{}) (VMResult, error) {
	log.V(1).Info("Sending WinRM command", "command", command, "args", args,
		"cmdline", fmt.Sprintf(command+"\r\n", args...))
	_, err := fmt.Fprintf(cmd.Stdin, command+"\r\n", args...)
	if err != nil {
		return VMResult{}, err
	}
	return getWinrmResult(cmd)
}

type CloudInitFile struct {
	Filename string
	Contents []byte
}

func writeCloudInit(log logr.Logger, hostname, sharePath string, bootstrapData, metaData, networkConfig []byte, networking *infrav1.ScvmmMachineNetworking) error {
	log.V(1).Info("Writing cloud-init", "sharePath", sharePath)
	// Parse share path into hostname, sharename, path
	shareParts := strings.Split(sharePath, "\\")
	host := shareParts[2]
	share := shareParts[3]
	path := strings.Join(shareParts[4:], "/")

	log.V(1).Info("smb2 Connecting", "host", host, "port", 445)
	conn, err := net.Dial("tcp", host+":445")
	if err != nil {
		return err
	}
	defer conn.Close()

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     ScvmmUsername,
			Password: ScvmmPassword,
		},
	}

	log.V(1).Info("smb2 Dialing", "user", ScvmmUsername)
	s, err := d.Dial(conn)
	if err != nil {
		return err
	}
	defer s.Logoff()

	log.V(1).Info("smb2 Mounting share", "share", share)
	fs, err := s.Mount(share)
	if err != nil {
		return err
	}
	defer fs.Umount()
	log.V(1).Info("smb2 Creating file", "path", path)
	fh, err := fs.Create(path)
	if err != nil {
		return err
	}
	if metaData == nil {
		data := "instance-id: scvmm-capi\n" +
			"hostname: " + hostname + "\n"
		metaData = []byte(data)
	}
	if networkConfig == nil && networking != nil {
		data := "version: 2\n" +
			"ethernets:\n" +
			"  eth0:\n" +
			"    addresses:\n" +
			"    - " + networking.IPAddress + "\n" +
			"    gateway4: " + networking.Gateway + "\n" +
			"    nameservers:\n" +
			"      search:\n" +
			"      - " + networking.Domain + "\n" +
			"      addresses:\n" +
			"      - " + strings.Join(networking.Nameservers, "\n      - ") + "\n"
		networkConfig = []byte(data)
	}
	numFiles := 2
	if networkConfig != nil {
		numFiles = 3
	}
	files := make([]CloudInitFile, numFiles)
	files[0] = CloudInitFile{
		"meta-data",
		metaData,
	}
	files[1] = CloudInitFile{
		"user-data",
		bootstrapData,
	}
	if networkConfig != nil {
		files[2] = CloudInitFile{
			"network-config",
			networkConfig,
		}
	}
	log.V(1).Info("smb2 Writing ISO", "path", path)
	if err := writeISO9660(fh, files); err != nil {
		log.Error(err, "Writing ISO file", "host", host, "share", share, "path", path)
		fh.Close()
		fs.Remove(path)
		return err
	}
	log.V(1).Info("smb2 Closing file")
	fh.Close()
	return nil
}

func putString(buf []byte, text string) {
	const padString = "                                                                                                                                "
	copy(buf, []byte(text+padString))
}

func putU16(buf []byte, value uint16) {
	binary.LittleEndian.PutUint16(buf[0:2], value)
	binary.BigEndian.PutUint16(buf[2:4], value)
}

func putU32(buf []byte, value uint32) {
	binary.LittleEndian.PutUint32(buf[0:4], value)
	binary.BigEndian.PutUint32(buf[4:8], value)
}

func putDate(buf []byte, value time.Time) {
	if value.IsZero() {
		copy(buf[0:16], []byte("0000000000000000"))
	} else {
		copy(buf[0:16], []byte(value.UTC().Format("2006010215040500")))
	}
	buf[16] = 0
}

type isoDirent struct {
	Location      int
	Length        int
	RecordingDate time.Time
	FileFlags     byte
	Identifier    string
}

func putDirent(sector []byte, offset int, dirent *isoDirent) int {
	identLen := len(dirent.Identifier)
	totlen := (33 + identLen | 1) + 1 // Pad to even length
	if offset+totlen > 2048 {
		return -1
	}
	buf := sector[offset : offset+totlen]
	buf[0] = byte(totlen)
	putU32(buf[2:10], uint32(dirent.Location))
	putU32(buf[10:18], uint32(dirent.Length))

	year, month, day := dirent.RecordingDate.UTC().Date()
	hour, minute, second := dirent.RecordingDate.UTC().Clock()
	buf[18] = byte(year - 1900)
	buf[19] = byte(month)
	buf[20] = byte(day)
	buf[21] = byte(hour)
	buf[22] = byte(minute)
	buf[23] = byte(second)

	buf[25] = dirent.FileFlags
	putU16(buf[28:32], 1) // Volume sequence number
	buf[32] = byte(identLen)
	if identLen > 0 {
		copy(buf[33:256], []byte(dirent.Identifier))
	}
	return offset + totlen
}

func writeISO9660(fh *smb2.File, files []CloudInitFile) error {
	sector := make([]byte, 2048)
	now := time.Now()

	// Calculate the total size
	// NB: Assumes all files are in the root and the dirent will not exceed one sector
	// 16,17 = volume identifiers, 18 = directory
	lastSector := 19
	for cif := range files {
		// Round up to sector size
		fsz := ((len(files[cif].Contents) - 1) / 2048) + 1
		lastSector = lastSector + fsz
	}

	// Start with 32K of zeroes
	for i := 0; i < 16; i++ {
		if _, err := fh.Write(sector); err != nil {
			return err
		}
	}
	// Write Primary Volume Descriptor (sector 16)
	sector[0] = 1
	putString(sector[1:6], "CD001")
	sector[7] = 1
	putString(sector[8:40], "LINUX")                                        // System identifier
	putString(sector[40:72], "cidata")                                      // Volume identifier
	putU32(sector[80:88], uint32(lastSector))                               // Volume Space Size
	putU16(sector[120:124], 1)                                              // Volume Set Size
	putU16(sector[124:128], 1)                                              // Sequence Number
	putU16(sector[128:132], 2048)                                           // Logical Block Size
	putDirent(sector, 156, &isoDirent{18, 2048, now, 2, string([]byte{0})}) // Root directory entry
	putString(sector[190:318], "")                                          // Volume Set
	putString(sector[318:446], "")                                          // Publisher
	putString(sector[446:574], "")                                          // Data Preparer
	putString(sector[574:702], "cluster-api-provider-scvmm")                // Application
	putString(sector[702:740], "")                                          // Copyright File
	putString(sector[740:776], "")                                          // Abstract File
	putString(sector[776:813], "")                                          // Bibliographic File
	putDate(sector[813:830], now)                                           // Volume Creation
	putDate(sector[830:847], now)                                           // Volume Modification
	putDate(sector[847:864], time.Time{})                                   // Volume Expiration
	putDate(sector[864:881], now)                                           // Volume Effective
	sector[881] = 1                                                         // File Structure Version
	if _, err := fh.Write(sector); err != nil {
		return err
	}

	for i := range sector {
		sector[i] = 0
	}
	// Write Terminator VD (sector 17)
	sector[0] = 255                    // Type
	copy(sector[1:6], []byte("CD001")) // Identifier
	sector[6] = 1                      // Version
	if _, err := fh.Write(sector); err != nil {
		return err
	}
	for i := range sector {
		sector[i] = 0
	}

	// Write directory (sector 18)
	curOff := 0
	curOff = putDirent(sector, curOff, &isoDirent{18, 2048, now, 2, string([]byte{0})}) // Own directory entry
	curOff = putDirent(sector, curOff, &isoDirent{18, 2048, now, 2, string([]byte{1})}) // Parent directory entry

	// Write directory entries
	fileSector := 19
	for cif := range files {
		flen := len(files[cif].Contents)
		curOff = putDirent(sector, curOff, &isoDirent{fileSector, flen, now, 0, files[cif].Filename + ";1"})
		fileSector = fileSector + ((flen - 1) / 2048) + 1
	}
	if _, err := fh.Write(sector); err != nil {
		return err
	}
	for i := range sector {
		sector[i] = 0
	}
	for cif := range files {
		if _, err := fh.Write(files[cif].Contents); err != nil {
			return err
		}
		padlen := (2048 - (len(files[cif].Contents) % 2048)) % 2048
		if padlen > 0 {
			if _, err := fh.Write(sector[:padlen]); err != nil {
				return err
			}
		}
	}
	return nil
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;machines,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

func (r *ScvmmMachineReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("scvmmmachine", req.NamespacedName)

	log.V(1).Info("Fetching scvmmmachine")
	// Fetch the instance
	scvmmMachine := &infrav1.ScvmmMachine{}
	if err := r.Get(ctx, req.NamespacedName, scvmmMachine); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	patchHelper, err := patch.NewHelper(scvmmMachine, r)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "Get patchhelper")
	}

	// Handle deleted machines
	// NB: The reference implementation handles deletion at the end of this function, but that seems wrogn
	//     because it could be that the machine, cluster, etc is gone and also it tries to add finalizers
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
			return patchReasonCondition(ctx, log, patchHelper, scvmmMachine, 0, nil, VmCreated, WaitingForOwnerReason, "")
		}

		log = log.WithValues("machine", machine.Name)

		log.V(1).Info("Fetching cluster")
		// Fetch the Cluster.
		cluster, err = util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
		if err != nil {
			log.Info("ScvmmMachine owner Machine is missing cluster label or cluster does not exist")
			return patchReasonCondition(ctx, log, patchHelper, scvmmMachine, 0, err, VmCreated, MissingClusterReason, "ScvmmMachine owner Machine is missing cluster label or cluster does not exist")
		}
		if cluster == nil {
			log.Info(fmt.Sprintf("Please associate this machine with a cluster using the label %s: <name of cluster>", clusterv1.ClusterLabelName))
			return patchReasonCondition(ctx, log, patchHelper, scvmmMachine, 0, nil, VmCreated, MissingClusterReason, "Please associate this machine with a cluster using the label %s: <name of cluster>", clusterv1.ClusterLabelName)
		}

		log = log.WithValues("cluster", cluster.Name)

		log.V(1).Info("Fetching scvmmcluster")
		// Fetch the Scvmm Cluster.
		scvmmCluster := &infrav1.ScvmmCluster{}
		scvmmClusterName := client.ObjectKey{
			Namespace: scvmmMachine.Namespace,
			Name:      cluster.Spec.InfrastructureRef.Name,
		}
		if err := r.Client.Get(ctx, scvmmClusterName, scvmmCluster); err != nil {
			log.Info("ScvmmCluster is not available yet")
			return patchReasonCondition(ctx, log, patchHelper, scvmmMachine, 0, nil, VmCreated, ClusterNotAvailableReason, "")
		}

		log = log.WithValues("scvmm-cluster", scvmmCluster.Name)

		// Check if the infrastructure is ready, otherwise return and wait for the cluster object to be updated
		if !cluster.Status.InfrastructureReady {
			log.Info("Waiting for ScvmmCluster Controller to create cluster infrastructure")
			return patchReasonCondition(ctx, log, patchHelper, scvmmMachine, 0, nil, VmCreated, WaitingForClusterInfrastructureReason, "")
		}
	}

	log.V(1).Info("Check finalizer")
	// Add finalizer.  Apparently we should return here to avoid a race condition
	// (Presumably the change/patch will trigger another reconciliation so it continues)
	if !controllerutil.ContainsFinalizer(scvmmMachine, MachineFinalizer) {
		log.V(1).Info("Add finalizer")
		controllerutil.AddFinalizer(scvmmMachine, MachineFinalizer)
		if perr := patchScvmmMachine(ctx, patchHelper, scvmmMachine); perr != nil {
			log.Error(perr, "Failed to patch scvmmMachine", "scvmmmachine", scvmmMachine)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, patchHelper, cluster, machine, scvmmMachine)
}

func (r *ScvmmMachineReconciler) reconcileNormal(ctx context.Context, patchHelper *patch.Helper, cluster *clusterv1.Cluster, machine *clusterv1.Machine, scvmmMachine *infrav1.ScvmmMachine) (res ctrl.Result, retErr error) {
	log := r.Log.WithValues("scvmmmachine", scvmmMachine.Name)

	log.Info("Doing reconciliation of ScvmmMachine")
	cmd, err := createWinrmCmd()
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "Winrm")
	}
	defer cmd.Close()
	log.V(1).Info("Running GetVM")
	vm, err := sendWinrmCommand(log, cmd, "GetVM -VMName '%s'", scvmmMachine.Spec.VMName)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "Failed to get vm")
	}
	log.V(1).Info("GetVM result", "vm", vm)
	if vm.Name == "" {
		log.V(1).Info("Call CreateVM")
		spec := scvmmMachine.Spec
		vm, err = sendWinrmCommand(log, cmd, "CreateVM -Cloud '%s' -HostGroup '%s' -VMName '%s' -VMTemplate '%s' -VHDisk '%s' -Memory %d -CPUCount %d -DiskSize %d -VMNetwork '%s' -HardwareProfile '%s' -Description '%s' -StartAction '%s' -StopAction '%s'",
			spec.Cloud, spec.HostGroup, spec.VMName,
			spec.VMTemplate, spec.VHDisk,
			(spec.Memory.Value() / 1024 / 1024),
			spec.CPUCount, (spec.DiskSize.Value() / 1024 / 1024),
			spec.VMNetwork, spec.HardwareProfile, spec.Description,
			spec.StartAction, spec.StopAction)
		if err != nil {
			return patchReasonCondition(ctx, log, patchHelper, scvmmMachine, 0, err, VmCreated, VmFailedReason, "Failed to create vm")
		}
		log.V(1).Info("CreateVM Result", "vm", vm)
		if vm.Error != "" {
			return patchReasonCondition(ctx, log, patchHelper, scvmmMachine, 0, nil, VmCreated, VmFailedReason, "Failed to create vm: %s", vm.Error)
		}

		log.V(1).Info("Fill in status")
		scvmmMachine.Spec.ProviderID = "scvmm://" + vm.Guid
		scvmmMachine.Status.Ready = false
		scvmmMachine.Status.VMStatus = vm.Status
		scvmmMachine.Status.BiosGuid = vm.Guid
		scvmmMachine.Status.CreationTime = vm.CreationTime
		scvmmMachine.Status.ModifiedTime = vm.ModifiedTime
		return patchReasonCondition(ctx, log, patchHelper, scvmmMachine, 10, nil, VmCreated, VmCreatingReason, "")
	}
	conditions.MarkTrue(scvmmMachine, VmCreated)

	log.V(1).Info("Machine is there, fill in status")
	scvmmMachine.Spec.ProviderID = "scvmm://" + vm.Guid
	scvmmMachine.Status.Ready = (vm.Status == "Running")
	scvmmMachine.Status.VMStatus = vm.Status
	scvmmMachine.Status.BiosGuid = vm.Guid
	scvmmMachine.Status.CreationTime = vm.CreationTime
	scvmmMachine.Status.ModifiedTime = vm.ModifiedTime

	if vm.Status == "PowerOff" {
		var bootstrapData, metaData, networkConfig []byte
		if machine != nil {
			if machine.Spec.Bootstrap.DataSecretName == nil {
				if !util.IsControlPlaneMachine(machine) && !cluster.Status.ControlPlaneInitialized {
					log.Info("Waiting for the control plane to be initialized")
					return patchReasonCondition(ctx, log, patchHelper, scvmmMachine, 0, nil, VmCreated, WaitingForControlPlaneAvailableReason, "")
				}
				log.Info("Waiting for the Bootstrap provider controller to set bootstrap data")
				return patchReasonCondition(ctx, log, patchHelper, scvmmMachine, 0, nil, VmCreated, WaitingForBootstrapDataReason, "")
			}
			log.V(1).Info("Get bootstrap data")
			bootstrapData, err = r.getBootstrapData(ctx, machine)
			if err != nil {
				r.Log.Error(err, "failed to get bootstrap data")
				return patchReasonCondition(ctx, log, patchHelper, scvmmMachine, 0, err, VmCreated, WaitingForBootstrapDataReason, "Failed to get bootstrap data")
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
		}
		if metaData != nil || bootstrapData != nil {
			log.V(1).Info("Create cloudinit")
			isoPath := ScvmmLibraryShare + "\\" + scvmmMachine.Spec.VMName + "-cloud-init.iso"
			if err := writeCloudInit(log, scvmmMachine.Spec.VMName, isoPath, bootstrapData, metaData, networkConfig, scvmmMachine.Spec.Networking); err != nil {
				r.Log.Error(err, "failed to create cloud init")
				return patchReasonCondition(ctx, log, patchHelper, scvmmMachine, 0, err, VmCreated, WaitingForBootstrapDataReason, "Failed to create cloud init data")
			}
			conditions.MarkFalse(scvmmMachine, VmRunning, VmStartingReason, clusterv1.ConditionSeverityInfo, "")
			if perr := patchScvmmMachine(ctx, patchHelper, scvmmMachine); perr != nil {
				log.Error(perr, "Failed to patch scvmmMachine", "scvmmmachine", scvmmMachine)
				return ctrl.Result{}, err
			}
			log.V(1).Info("Call AddIsoToVM")
			vm, err = sendWinrmCommand(log, cmd, "AddIsoToVM -VMName '%s' -ISOPath '%s'", scvmmMachine.Spec.VMName, isoPath)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "Failed to add iso to vm")
			}
			log.V(1).Info("AddIsoToVM result", "vm", vm)
		} else {
			log.V(1).Info("Call StartVM")
			vm, err = sendWinrmCommand(log, cmd, "StartVM -VMName '%s'", scvmmMachine.Spec.VMName)
			if err != nil {
				return ctrl.Result{}, errors.Wrap(err, "Failed to start vm")
			}
			log.V(1).Info("StartVM result", "vm", vm)
		}
		scvmmMachine.Status.VMStatus = vm.Status
		if perr := patchScvmmMachine(ctx, patchHelper, scvmmMachine); perr != nil {
			log.Error(perr, "Failed to patch scvmmMachine", "scvmmmachine", scvmmMachine)
			return ctrl.Result{}, err
		}
		log.V(1).Info("Requeue in 10 seconds")
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	// Wait for machine to get running state
	if vm.Status != "Running" {
		log.V(1).Info("Not running, Requeue in 30 seconds")
		return patchReasonCondition(ctx, log, patchHelper, scvmmMachine, 30, nil, VmRunning, VmStartingReason, "")
	}
	log.V(1).Info("Running, set status true")
	scvmmMachine.Status.Ready = true
	if vm.IPv4Addresses != nil {
		scvmmMachine.Status.Addresses = make([]clusterv1.MachineAddress, len(vm.IPv4Addresses))
		for i := range vm.IPv4Addresses {
			scvmmMachine.Status.Addresses[i] = clusterv1.MachineAddress{
				Type:    clusterv1.MachineInternalIP,
				Address: vm.IPv4Addresses[i],
			}
		}
	}
	conditions.MarkTrue(scvmmMachine, VmRunning)
	if perr := patchScvmmMachine(ctx, patchHelper, scvmmMachine); perr != nil {
		log.Error(perr, "Failed to patch scvmmMachine", "scvmmmachine", scvmmMachine)
		return ctrl.Result{}, err
	}
	if vm.IPv4Addresses == nil {
		log.V(1).Info("Call ReadVM")
		vm, err = sendWinrmCommand(log, cmd, "ReadVM -VMName '%s'", scvmmMachine.Spec.VMName)
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "Failed to read vm")
		}
		log.V(1).Info("ReadVM result", "vm", vm)
		log.Info("Reading vm IP addresses, reschedule after 60 seconds")
		return ctrl.Result{RequeueAfter: time.Second * 60}, nil
	}
	log.V(1).Info("Done")
	return ctrl.Result{}, nil
}

func (r *ScvmmMachineReconciler) reconcileDelete(ctx context.Context, patchHelper *patch.Helper, scvmmMachine *infrav1.ScvmmMachine) (ctrl.Result, error) {
	log := r.Log.WithValues("scvmmmachine", scvmmMachine.Name)

	log.V(1).Info("Do delete reconciliation")
	// If there's no finalizer do nothing
	if !controllerutil.ContainsFinalizer(scvmmMachine, MachineFinalizer) {
		return ctrl.Result{}, nil
	}
	log.V(1).Info("Set created to false, doing deletion")
	conditions.MarkFalse(scvmmMachine, VmCreated, VmDeletingReason, clusterv1.ConditionSeverityInfo, "")
	if err := patchScvmmMachine(ctx, patchHelper, scvmmMachine); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to patch ScvmmMachine")
	}

	log.Info("Doing removal of ScvmmMachine")
	cmd, err := createWinrmCmd()
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "Winrm")
	}
	defer cmd.Close()

	log.V(1).Info("Call RemoveVM")
	vm, err := sendWinrmCommand(log, cmd, "RemoveVM -VMName '%s'", scvmmMachine.Spec.VMName)
	if err != nil {
		return patchReasonCondition(ctx, log, patchHelper, scvmmMachine, 0, err, VmCreated, VmFailedReason, "Failed to delete VM")
	}
	log.V(1).Info("RemoveVM Result", "vm", vm)
	if vm.Error != "" {
		return patchReasonCondition(ctx, log, patchHelper, scvmmMachine, 60, nil, VmCreated, VmFailedReason, "Failed to delete VM: %s", vm.Error)
	}
	if vm.Message == "Removed" {
		log.V(1).Info("Machine is removed, remove finalizer")
		controllerutil.RemoveFinalizer(scvmmMachine, MachineFinalizer)
		if perr := patchScvmmMachine(ctx, patchHelper, scvmmMachine); perr != nil {
			log.Error(perr, "Failed to patch scvmmMachine", "scvmmmachine", scvmmMachine)
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	} else {
		log.V(1).Info("Set status")
		scvmmMachine.Status.VMStatus = vm.Status
		scvmmMachine.Status.CreationTime = vm.CreationTime
		scvmmMachine.Status.ModifiedTime = vm.ModifiedTime
		log.V(1).Info("Requeue after 30 seconds")
		return patchReasonCondition(ctx, log, patchHelper, scvmmMachine, 30, err, VmCreated, VmDeletingReason, "")
	}
}

func (r *ScvmmMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ScvmmHost = os.Getenv("SCVMM_HOST")
	if ScvmmHost == "" {
		return fmt.Errorf("missing required env SCVMM_HOST")
	}
	ScvmmExecHost = os.Getenv("SCVMM_EXECHOST")
	ScriptDir = os.Getenv("SCRIPT_DIR")
	ScvmmLibraryShare = os.Getenv("SCVMM_LIBRARY")

	ScvmmUsername = os.Getenv("SCVMM_USERNAME")
	if ScvmmUsername == "" {
		return fmt.Errorf("missing required env SCVMM_USERNAME")
	}
	ScvmmPassword = os.Getenv("SCVMM_PASSWORD")
	if ScvmmPassword == "" {
		return fmt.Errorf("missing required env SCVMM_PASSWORD")
	}

	funcScript := ""
	if ScvmmExecHost != "" {
		data, err := ioutil.ReadFile(ScriptDir + "/init.ps1")
		if err != nil {
			return errors.Wrap(err, "Read init.ps1")
		}
		funcScript = os.Expand(string(data), func(key string) string {
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
		return errors.Wrap(err, "Read functions.ps1")
	}
	funcScript = funcScript + string(data)
	FunctionScript = []byte(funcScript)

	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.ScvmmMachine{}).
		Complete(r)
}

func patchReasonCondition(ctx context.Context, log logr.Logger, patchHelper *patch.Helper, scvmmMachine *infrav1.ScvmmMachine, requeue int, err error, condition clusterv1.ConditionType, reason string, message string, messageargs ...interface{}) (ctrl.Result, error) {
	scvmmMachine.Status.Ready = false
	if err != nil {
		conditions.MarkFalse(scvmmMachine, condition, reason, clusterv1.ConditionSeverityError, message, messageargs...)
	} else {
		conditions.MarkFalse(scvmmMachine, condition, reason, clusterv1.ConditionSeverityInfo, message, messageargs...)
	}
	if perr := patchScvmmMachine(ctx, patchHelper, scvmmMachine); perr != nil {
		log.Error(perr, "Failed to patch scvmmMachine", "scvmmmachine", scvmmMachine)
	}
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, reason)
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
