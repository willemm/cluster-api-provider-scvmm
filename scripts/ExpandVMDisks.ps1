param($vmname, $disks)
try {
  $disklist = $disks | ConvertFrom-Json
  foreach ($vhdisk in Get-SCVirtualDiskDrive -vm $vmname) {
    $lun = $vhdisk.LUN
    if ((($disklist[$lun].sizeMB - 1) * 1024 * 1024) -gt $vhdisk.VirtualHardDisk.MaximumSize) {
      $vdd = Expand-SCVirtualDiskDrive -VirtualDiskDrive $vhdisk -VirtualHardDiskSizeGB ($disklist[$lun].sizeMB / 1024) -JobGroup $JobGroupID -RunAsynchronously
    }
  }
  $vm = Get-SCVirtualMachine -Name $vmname
  if (-not $vm) {
    throw "Virtual Machine $vmname not found"
  }
  return VMToJson $vm "Resizing"
} catch {
  ErrorToJson 'Expand VM Disks' $_
}
