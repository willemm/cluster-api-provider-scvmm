param($id, $disks)
try {
  $disklist = $disks | ConvertFrom-Json
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    throw "Virtual Machine $id not found"
  }
  foreach ($vhdisk in $vm.VirtualDiskDrives) {
    $lun = $vhdisk.LUN
    if ((($disklist[$lun].sizeMB - 1) * 1024 * 1024) -gt $vhdisk.VirtualHardDisk.MaximumSize) {
      $vdd = Expand-SCVirtualDiskDrive -VirtualDiskDrive $vhdisk -VirtualHardDiskSizeGB ($disklist[$lun].sizeMB / 1024) -RunAsynchronously
    }
  }
  return VMToJson $vm "Resizing"
} catch {
  ErrorToJson 'Expand VM Disks' $_
}
