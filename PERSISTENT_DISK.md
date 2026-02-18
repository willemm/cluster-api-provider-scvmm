# Handling persistent disks

A persistent disk is a vhdx which will be re-attached to a different virtualmachine when a machine is deleted.  That way, storage can be persisted over node rollouts.

This is done by calling remove virtualdiskdrive with a retain option,
resulting in the vhdx file remaining on the hyper-v node's disk.
scvmm does not keep track of this file, so currently it is being kept track of by storing the path in a custom resource.

The persistent vhdx should be attached to a temporary virtualmachine (which will never be turned on), so that it can be found and manipulated through scvmm, and is also safe(r) from accidental deletion or cleanup.

One specific type of manipulation would be to migrate the temp-vm to whatever hyper-v node the newly created machine lives on, so that it is not relying on clustersharedvolumes being shared with the targeted hyper-v node.

Need to figure out if it is possible to take the scvmm virtualharddisk object when its virtualharddisk is being removed, and use that to re-attach it to the temporary vm (or vice versa), which would be the most desirable option as it doesn't need to faff around with remote paths and such.
