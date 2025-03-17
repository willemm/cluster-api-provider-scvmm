param($id, $disks)
try {
  $disklist = $disks | ConvertFrom-Json
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    throw "Virtual Machine $id not found"
  }
  $seenluns = @{}
  foreach ($vhdisk in $vm.VirtualDiskDrives) {
    $lun = $vhdisk.LUN
    $seenluns[$lun] = $true
    if ((($disklist[$lun].sizeMB - 1) * 1024 * 1024) -gt $vhdisk.VirtualHardDisk.MaximumSize) {
      $vdd = Expand-SCVirtualDiskDrive -VirtualDiskDrive $vhdisk -VirtualHardDiskSizeGB ($disklist[$lun].sizeMB / 1024) -RunAsynchronously
      return VMToJson $vm "Expanding"
    }
    if ($disklist[$lun].iopsMaximum -ne $vhdisk.IOPSMaximum) {
      $vdd = Set-SCVirtualDiskDrive -VirtualDiskDrive $vhdisk -IOPSMaximum $disklist[$lun].iopsMaximum -RunAsynchronously
      return VMToJson $vm "Setting IOPSMaximum"
    }
  }
  for ($lun = 0; $lun -lt $disklist.Length; $lun++) {
    if (-not $seenluns[$lun]) {
      $vdd = CreateVHD -disk $disklist[$lun] -lun $lun -vm $vm
      return VMToJson $vm "Adding Disk"
    }
  }

  return VMToJson $vm "Nothing"
} catch {
  ErrorToJson 'Expand VM Disks' $_
}
