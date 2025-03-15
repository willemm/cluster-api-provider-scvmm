param($id, $disks)
try {
  $disklist = $disks | ConvertFrom-Json
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    throw "Virtual Machine $id not found"
  }
  # TODO: Do this asjob maybe ?  And also add disks that are not present (for persistent disks)
  foreach ($vhdisk in $vm.VirtualDiskDrives) {
    $lun = $vhdisk.LUN
    if ((($disklist[$lun].sizeMB - 1) * 1024 * 1024) -gt $vhdisk.VirtualHardDisk.MaximumSize) {
      $vdd = Expand-SCVirtualDiskDrive -VirtualDiskDrive $vhdisk -VirtualHardDiskSizeGB ($disklist[$lun].sizeMB / 1024) -RunAsynchronously
    }
    if ($disklist[$lun].iopsMaximum -ne $vhdisk.IOPSMaximum) {
      $vdd = Set-SCVirtualDiskDrive -VirtualDiskDrive $vhdisk -IOPSMaximum $disklist[$lun].iopsMaximum -RunAsynchronously
    }
  }
  return VMToJson $vm "Resizing"
} catch {
  ErrorToJson 'Expand VM Disks' $_
}
