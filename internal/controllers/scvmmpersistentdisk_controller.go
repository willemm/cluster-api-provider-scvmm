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
	"errors"
	"net"
	"os"
	"strings"
	"time"

	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/hirochachacha/go-smb2"
	infrav1 "github.com/willemm/cluster-api-provider-scvmm/api/v1alpha1"
)

// ScvmmPersistentDiskReconciler reconciles a ScvmmPersistentDisk object
type ScvmmPersistentDiskReconciler struct {
	client.Client
}

const (
	PersistentDiskFinalizer = "scvmmpersistentdisk.finalizers.cluster.x-k8s.io"
)

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmpersistentdisks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmpersistentdisks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmpersistentdisks/finalizers,verbs=update
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmpersistentdiskpools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmpersistentdiskpools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scvmmpersistentdiskpools/finalizers,verbs=update

func (r *ScvmmPersistentDiskReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	log.V(1).Info("Fetching scvmmpersistentdisk")
	disk := &infrav1.ScvmmPersistentDisk{}
	if err := r.Get(ctx, req.NamespacedName, disk); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	log = log.WithValues("scvmmpersistentdisk", disk.Name)
	ctx = ctrl.LoggerInto(ctx, log)
	log.V(1).Info("Get provider")
	provider, err := getProvider(disk.Spec.ProviderRef)
	if err != nil {
		log.Error(err, "Failed to get provider")
		return ctrl.Result{}, err
	}

	diskShare, err := getPersistentDiskFromShare(ctx, disk, provider)
	if err != nil {
		log.Error(err, "Failed to read persistentdisk share")
		return ctrl.Result{}, err
	}

	log.Info("Got disk from share", "disk", disk, "diskShare", diskShare)
	if !disk.DeletionTimestamp.IsZero() {
		if !diskShare.Exists {
			// Disk not found, so assume it's gone and remove the finalizer
			log.V(1).Info("diskShare not found, remove finalizer")
			controllerutil.RemoveFinalizer(disk, PersistentDiskFinalizer)
			if err := client.IgnoreNotFound(r.Update(ctx, disk)); err != nil {
				log.Error(err, "Failed to update scvmmMachine", "disk", disk)
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		vm, err := sendWinrmCommand(log, disk.Spec.ProviderRef, "RemovePersistentDisk -VMHost '%s' -Path '%s' -Filename '%s'",
			escapeSingleQuotes(disk.Spec.VMHost),
			escapeSingleQuotes(disk.Spec.Path),
			escapeSingleQuotes(disk.Spec.Filename),
		)
		if err != nil {
			log.Error(err, "Failed to run persistentdisk remove script", "disk", disk)
			return ctrl.Result{}, err
		}
		log.Info("Removing persistentdisk result", "result", vm)
		return ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	// TODO: Check if not owned, if so check if a machine exists that wants this

	return ctrl.Result{}, nil

	// ScvmmPersistentDisk is like a statefulset; the naming is sequentially numbered
	// Which means that it is easy to concurrently check for free slots
	// If the maxDisks setting is ever decreased, that means there can be
	// persistentdisk resources that will never be used.
	// (Maybe set error status if there are more persistentdisk resources than maxdisks)

	////// Scvmmmachine flow:
	// When creating disk that references diskpool, or triggered by unowned
	//
	// Get disk from diskpool that doesn't have machine owner and set owner to machine
	//  (retry if setting owner fails)
	//
	// If no free disks, set disk number to largest existing plus one.
	//  if that's greater than maxdisks, do nothing and break
	//  else, create scvmmpersistent disk with that number (if that fails, redo from start)
	//
	// Use persistentdisk resource to add vdd to machine
	//  (if location is blank, create new and set location in resource)
}

type PersistentDiskShare struct {
	Host   string
	Share  string
	Path   string
	Size   int64
	Exists bool
}

func getPersistentDiskFromShare(ctx context.Context, disk *infrav1.ScvmmPersistentDisk, provider *infrav1.ScvmmProviderSpec) (*PersistentDiskShare, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("Finding disk in share")
	// Parse share path into sharename, path
	shareParts := append(strings.Split(disk.Spec.Path, "\\"), disk.Spec.Filename)
	// Translate C: to C$ because that's how it's shared
	diskShare := &PersistentDiskShare{
		Host:  disk.Spec.VMHost,
		Share: strings.Replace(shareParts[0], ":", "$", -1),
		Path:  strings.Join(shareParts[1:], "/"),
	}
	if disk.Spec.Path == "" {
		diskShare.Exists = false
		return diskShare, nil
	}

	log.V(1).Info("smb2 Connecting", "host", diskShare.Host, "port", 445)
	conn, err := net.Dial("tcp", diskShare.Host+":445")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	userParts := strings.Split(provider.ScvmmUsername, "\\")

	smbCreds := &smb2.NTLMInitiator{
		User:     userParts[0],
		Password: provider.ScvmmPassword,
	}
	if len(userParts) > 1 {
		smbCreds.Domain = userParts[0]
		smbCreds.User = userParts[1]
	}
	d := &smb2.Dialer{Initiator: smbCreds}

	log.V(1).Info("smb2 Dialing", "user", provider.ScvmmUsername)
	s, err := d.Dial(conn)
	if err != nil {
		return nil, err
	}
	defer s.Logoff()

	log.V(1).Info("smb2 Mounting share", "share", diskShare.Share)
	fs, err := s.Mount(diskShare.Share)
	if err != nil {
		return nil, err
	}
	defer fs.Umount()

	fstat, err := fs.Stat(diskShare.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.V(1).Info("smb2 got error ErrNotExist")
			diskShare.Size = 0
			diskShare.Exists = false
			return diskShare, nil
		}
		return nil, err
	}
	log.V(1).Info("smb2 got stat", "fstat", fstat)

	diskShare.Size = fstat.Size()
	diskShare.Exists = true
	return diskShare, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScvmmPersistentDiskReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	log := ctrl.LoggerFrom(ctx)
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.ScvmmPersistentDisk{}).
		WithEventFilter(predicate.And(
			predicates.ResourceNotPaused(log),
		)).
		Complete(r)
}
