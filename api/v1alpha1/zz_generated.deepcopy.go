//go:build !ignore_autogenerated

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

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/api/v1beta1"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ActiveDirectory) DeepCopyInto(out *ActiveDirectory) {
	*out = *in
	if in.MemberOf != nil {
		in, out := &in.MemberOf, &out.MemberOf
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ActiveDirectory.
func (in *ActiveDirectory) DeepCopy() *ActiveDirectory {
	if in == nil {
		return nil
	}
	out := new(ActiveDirectory)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CloudInit) DeepCopyInto(out *CloudInit) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CloudInit.
func (in *CloudInit) DeepCopy() *CloudInit {
	if in == nil {
		return nil
	}
	out := new(CloudInit)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DynamicMemory) DeepCopyInto(out *DynamicMemory) {
	*out = *in
	if in.Minimum != nil {
		in, out := &in.Minimum, &out.Minimum
		x := (*in).DeepCopy()
		*out = &x
	}
	if in.Maximum != nil {
		in, out := &in.Maximum, &out.Maximum
		x := (*in).DeepCopy()
		*out = &x
	}
	if in.BufferPercentage != nil {
		in, out := &in.BufferPercentage, &out.BufferPercentage
		*out = new(int)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DynamicMemory.
func (in *DynamicMemory) DeepCopy() *DynamicMemory {
	if in == nil {
		return nil
	}
	out := new(DynamicMemory)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *FibreChannel) DeepCopyInto(out *FibreChannel) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new FibreChannel.
func (in *FibreChannel) DeepCopy() *FibreChannel {
	if in == nil {
		return nil
	}
	out := new(FibreChannel)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *NetworkDevice) DeepCopyInto(out *NetworkDevice) {
	*out = *in
	if in.IPAddresses != nil {
		in, out := &in.IPAddresses, &out.IPAddresses
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Nameservers != nil {
		in, out := &in.Nameservers, &out.Nameservers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.SearchDomains != nil {
		in, out := &in.SearchDomains, &out.SearchDomains
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.AddressesFromPools != nil {
		in, out := &in.AddressesFromPools, &out.AddressesFromPools
		*out = make([]v1.TypedLocalObjectReference, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new NetworkDevice.
func (in *NetworkDevice) DeepCopy() *NetworkDevice {
	if in == nil {
		return nil
	}
	out := new(NetworkDevice)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Networking) DeepCopyInto(out *Networking) {
	*out = *in
	if in.Devices != nil {
		in, out := &in.Devices, &out.Devices
		*out = make([]NetworkDevice, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Networking.
func (in *Networking) DeepCopy() *Networking {
	if in == nil {
		return nil
	}
	out := new(Networking)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ObjectMeta) DeepCopyInto(out *ObjectMeta) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ObjectMeta.
func (in *ObjectMeta) DeepCopy() *ObjectMeta {
	if in == nil {
		return nil
	}
	out := new(ObjectMeta)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmCloudInitSpec) DeepCopyInto(out *ScvmmCloudInitSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmCloudInitSpec.
func (in *ScvmmCloudInitSpec) DeepCopy() *ScvmmCloudInitSpec {
	if in == nil {
		return nil
	}
	out := new(ScvmmCloudInitSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmCluster) DeepCopyInto(out *ScvmmCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmCluster.
func (in *ScvmmCluster) DeepCopy() *ScvmmCluster {
	if in == nil {
		return nil
	}
	out := new(ScvmmCluster)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ScvmmCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmClusterList) DeepCopyInto(out *ScvmmClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ScvmmCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmClusterList.
func (in *ScvmmClusterList) DeepCopy() *ScvmmClusterList {
	if in == nil {
		return nil
	}
	out := new(ScvmmClusterList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ScvmmClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmClusterSpec) DeepCopyInto(out *ScvmmClusterSpec) {
	*out = *in
	out.ControlPlaneEndpoint = in.ControlPlaneEndpoint
	if in.ProviderRef != nil {
		in, out := &in.ProviderRef, &out.ProviderRef
		*out = new(ScvmmProviderReference)
		**out = **in
	}
	if in.FailureDomains != nil {
		in, out := &in.FailureDomains, &out.FailureDomains
		*out = make(map[string]ScvmmFailureDomainSpec, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmClusterSpec.
func (in *ScvmmClusterSpec) DeepCopy() *ScvmmClusterSpec {
	if in == nil {
		return nil
	}
	out := new(ScvmmClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmClusterStatus) DeepCopyInto(out *ScvmmClusterStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(v1beta1.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.FailureDomains != nil {
		in, out := &in.FailureDomains, &out.FailureDomains
		*out = make(v1beta1.FailureDomains, len(*in))
		for key, val := range *in {
			(*out)[key] = *val.DeepCopy()
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmClusterStatus.
func (in *ScvmmClusterStatus) DeepCopy() *ScvmmClusterStatus {
	if in == nil {
		return nil
	}
	out := new(ScvmmClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmClusterTemplate) DeepCopyInto(out *ScvmmClusterTemplate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmClusterTemplate.
func (in *ScvmmClusterTemplate) DeepCopy() *ScvmmClusterTemplate {
	if in == nil {
		return nil
	}
	out := new(ScvmmClusterTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ScvmmClusterTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmClusterTemplateList) DeepCopyInto(out *ScvmmClusterTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ScvmmClusterTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmClusterTemplateList.
func (in *ScvmmClusterTemplateList) DeepCopy() *ScvmmClusterTemplateList {
	if in == nil {
		return nil
	}
	out := new(ScvmmClusterTemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ScvmmClusterTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmClusterTemplateResource) DeepCopyInto(out *ScvmmClusterTemplateResource) {
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmClusterTemplateResource.
func (in *ScvmmClusterTemplateResource) DeepCopy() *ScvmmClusterTemplateResource {
	if in == nil {
		return nil
	}
	out := new(ScvmmClusterTemplateResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmClusterTemplateSpec) DeepCopyInto(out *ScvmmClusterTemplateSpec) {
	*out = *in
	in.Template.DeepCopyInto(&out.Template)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmClusterTemplateSpec.
func (in *ScvmmClusterTemplateSpec) DeepCopy() *ScvmmClusterTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(ScvmmClusterTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmFailureDomainSpec) DeepCopyInto(out *ScvmmFailureDomainSpec) {
	*out = *in
	if in.Networking != nil {
		in, out := &in.Networking, &out.Networking
		*out = new(Networking)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmFailureDomainSpec.
func (in *ScvmmFailureDomainSpec) DeepCopy() *ScvmmFailureDomainSpec {
	if in == nil {
		return nil
	}
	out := new(ScvmmFailureDomainSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmMachine) DeepCopyInto(out *ScvmmMachine) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmMachine.
func (in *ScvmmMachine) DeepCopy() *ScvmmMachine {
	if in == nil {
		return nil
	}
	out := new(ScvmmMachine)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ScvmmMachine) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmMachineList) DeepCopyInto(out *ScvmmMachineList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ScvmmMachine, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmMachineList.
func (in *ScvmmMachineList) DeepCopy() *ScvmmMachineList {
	if in == nil {
		return nil
	}
	out := new(ScvmmMachineList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ScvmmMachineList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmMachineSpec) DeepCopyInto(out *ScvmmMachineSpec) {
	*out = *in
	if in.VMNameFromPool != nil {
		in, out := &in.VMNameFromPool, &out.VMNameFromPool
		*out = new(v1.LocalObjectReference)
		**out = **in
	}
	if in.Disks != nil {
		in, out := &in.Disks, &out.Disks
		*out = make([]VmDisk, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.FibreChannel != nil {
		in, out := &in.FibreChannel, &out.FibreChannel
		*out = make([]FibreChannel, len(*in))
		copy(*out, *in)
	}
	if in.Memory != nil {
		in, out := &in.Memory, &out.Memory
		x := (*in).DeepCopy()
		*out = &x
	}
	if in.DynamicMemory != nil {
		in, out := &in.DynamicMemory, &out.DynamicMemory
		*out = new(DynamicMemory)
		(*in).DeepCopyInto(*out)
	}
	if in.Networking != nil {
		in, out := &in.Networking, &out.Networking
		*out = new(Networking)
		(*in).DeepCopyInto(*out)
	}
	if in.ActiveDirectory != nil {
		in, out := &in.ActiveDirectory, &out.ActiveDirectory
		*out = new(ActiveDirectory)
		(*in).DeepCopyInto(*out)
	}
	if in.VMOptions != nil {
		in, out := &in.VMOptions, &out.VMOptions
		*out = new(VmOptions)
		(*in).DeepCopyInto(*out)
	}
	if in.CloudInit != nil {
		in, out := &in.CloudInit, &out.CloudInit
		*out = new(CloudInit)
		**out = **in
	}
	if in.CustomProperty != nil {
		in, out := &in.CustomProperty, &out.CustomProperty
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.ProviderRef != nil {
		in, out := &in.ProviderRef, &out.ProviderRef
		*out = new(ScvmmProviderReference)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmMachineSpec.
func (in *ScvmmMachineSpec) DeepCopy() *ScvmmMachineSpec {
	if in == nil {
		return nil
	}
	out := new(ScvmmMachineSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmMachineStatus) DeepCopyInto(out *ScvmmMachineStatus) {
	*out = *in
	in.CreationTime.DeepCopyInto(&out.CreationTime)
	in.ModifiedTime.DeepCopyInto(&out.ModifiedTime)
	if in.Addresses != nil {
		in, out := &in.Addresses, &out.Addresses
		*out = make([]v1beta1.MachineAddress, len(*in))
		copy(*out, *in)
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(v1beta1.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmMachineStatus.
func (in *ScvmmMachineStatus) DeepCopy() *ScvmmMachineStatus {
	if in == nil {
		return nil
	}
	out := new(ScvmmMachineStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmMachineTemplate) DeepCopyInto(out *ScvmmMachineTemplate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmMachineTemplate.
func (in *ScvmmMachineTemplate) DeepCopy() *ScvmmMachineTemplate {
	if in == nil {
		return nil
	}
	out := new(ScvmmMachineTemplate)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ScvmmMachineTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmMachineTemplateList) DeepCopyInto(out *ScvmmMachineTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ScvmmMachineTemplate, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmMachineTemplateList.
func (in *ScvmmMachineTemplateList) DeepCopy() *ScvmmMachineTemplateList {
	if in == nil {
		return nil
	}
	out := new(ScvmmMachineTemplateList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ScvmmMachineTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmMachineTemplateResource) DeepCopyInto(out *ScvmmMachineTemplateResource) {
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmMachineTemplateResource.
func (in *ScvmmMachineTemplateResource) DeepCopy() *ScvmmMachineTemplateResource {
	if in == nil {
		return nil
	}
	out := new(ScvmmMachineTemplateResource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmMachineTemplateSpec) DeepCopyInto(out *ScvmmMachineTemplateSpec) {
	*out = *in
	in.Template.DeepCopyInto(&out.Template)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmMachineTemplateSpec.
func (in *ScvmmMachineTemplateSpec) DeepCopy() *ScvmmMachineTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(ScvmmMachineTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmNamePool) DeepCopyInto(out *ScvmmNamePool) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmNamePool.
func (in *ScvmmNamePool) DeepCopy() *ScvmmNamePool {
	if in == nil {
		return nil
	}
	out := new(ScvmmNamePool)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ScvmmNamePool) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmNamePoolList) DeepCopyInto(out *ScvmmNamePoolList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ScvmmNamePool, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmNamePoolList.
func (in *ScvmmNamePoolList) DeepCopy() *ScvmmNamePoolList {
	if in == nil {
		return nil
	}
	out := new(ScvmmNamePoolList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ScvmmNamePoolList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmNamePoolSpec) DeepCopyInto(out *ScvmmNamePoolSpec) {
	*out = *in
	if in.VMNameRanges != nil {
		in, out := &in.VMNameRanges, &out.VMNameRanges
		*out = make([]VmNameRange, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmNamePoolSpec.
func (in *ScvmmNamePoolSpec) DeepCopy() *ScvmmNamePoolSpec {
	if in == nil {
		return nil
	}
	out := new(ScvmmNamePoolSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmNamePoolStatus) DeepCopyInto(out *ScvmmNamePoolStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(v1beta1.Conditions, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.VMNameOwners != nil {
		in, out := &in.VMNameOwners, &out.VMNameOwners
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Counts != nil {
		in, out := &in.Counts, &out.Counts
		*out = new(ScvmmPoolCounts)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmNamePoolStatus.
func (in *ScvmmNamePoolStatus) DeepCopy() *ScvmmNamePoolStatus {
	if in == nil {
		return nil
	}
	out := new(ScvmmNamePoolStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmPoolCounts) DeepCopyInto(out *ScvmmPoolCounts) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmPoolCounts.
func (in *ScvmmPoolCounts) DeepCopy() *ScvmmPoolCounts {
	if in == nil {
		return nil
	}
	out := new(ScvmmPoolCounts)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmProvider) DeepCopyInto(out *ScvmmProvider) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmProvider.
func (in *ScvmmProvider) DeepCopy() *ScvmmProvider {
	if in == nil {
		return nil
	}
	out := new(ScvmmProvider)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ScvmmProvider) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmProviderList) DeepCopyInto(out *ScvmmProviderList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]ScvmmProvider, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmProviderList.
func (in *ScvmmProviderList) DeepCopy() *ScvmmProviderList {
	if in == nil {
		return nil
	}
	out := new(ScvmmProviderList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ScvmmProviderList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmProviderReference) DeepCopyInto(out *ScvmmProviderReference) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmProviderReference.
func (in *ScvmmProviderReference) DeepCopy() *ScvmmProviderReference {
	if in == nil {
		return nil
	}
	out := new(ScvmmProviderReference)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmProviderSpec) DeepCopyInto(out *ScvmmProviderSpec) {
	*out = *in
	if in.ScvmmSecret != nil {
		in, out := &in.ScvmmSecret, &out.ScvmmSecret
		*out = new(v1.SecretReference)
		**out = **in
	}
	out.CloudInit = in.CloudInit
	if in.ADSecret != nil {
		in, out := &in.ADSecret, &out.ADSecret
		*out = new(v1.SecretReference)
		**out = **in
	}
	if in.ExtraFunctions != nil {
		in, out := &in.ExtraFunctions, &out.ExtraFunctions
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.SensitiveEnv != nil {
		in, out := &in.SensitiveEnv, &out.SensitiveEnv
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmProviderSpec.
func (in *ScvmmProviderSpec) DeepCopy() *ScvmmProviderSpec {
	if in == nil {
		return nil
	}
	out := new(ScvmmProviderSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ScvmmProviderStatus) DeepCopyInto(out *ScvmmProviderStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ScvmmProviderStatus.
func (in *ScvmmProviderStatus) DeepCopy() *ScvmmProviderStatus {
	if in == nil {
		return nil
	}
	out := new(ScvmmProviderStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VmDisk) DeepCopyInto(out *VmDisk) {
	*out = *in
	if in.Size != nil {
		in, out := &in.Size, &out.Size
		x := (*in).DeepCopy()
		*out = &x
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VmDisk.
func (in *VmDisk) DeepCopy() *VmDisk {
	if in == nil {
		return nil
	}
	out := new(VmDisk)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VmNameRange) DeepCopyInto(out *VmNameRange) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VmNameRange.
func (in *VmNameRange) DeepCopy() *VmNameRange {
	if in == nil {
		return nil
	}
	out := new(VmNameRange)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *VmOptions) DeepCopyInto(out *VmOptions) {
	*out = *in
	if in.CPULimitForMigration != nil {
		in, out := &in.CPULimitForMigration, &out.CPULimitForMigration
		*out = new(bool)
		**out = **in
	}
	if in.CPULimitFunctionality != nil {
		in, out := &in.CPULimitFunctionality, &out.CPULimitFunctionality
		*out = new(bool)
		**out = **in
	}
	if in.EnableNestedVirtualization != nil {
		in, out := &in.EnableNestedVirtualization, &out.EnableNestedVirtualization
		*out = new(bool)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new VmOptions.
func (in *VmOptions) DeepCopy() *VmOptions {
	if in == nil {
		return nil
	}
	out := new(VmOptions)
	in.DeepCopyInto(out)
	return out
}
