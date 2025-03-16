param($id, $disks)
try {
  $JobGroupID = [GUID]::NewGuid().ToString()
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
      $vdd = Expand-SCVirtualDiskDrive -VirtualDiskDrive $vhdisk -VirtualHardDiskSizeGB ($disklist[$lun].sizeMB / 1024) -JobGroup $JobGroupID
    }
    if ($disklist[$lun].iopsMaximum -ne $vhdisk.IOPSMaximum) {
      $vdd = Set-SCVirtualDiskDrive -VirtualDiskDrive $vhdisk -IOPSMaximum $disklist[$lun].iopsMaximum -JobGroup $JobGroupID
    }
  }
  for ($lun = 0; $lun -lt $disklist.Length; $lun++) {
    if (-not $seen[$lun]) {
      $vdd = CreateVHD -disk $disklist[$lun] -lun $lun -vm $vm -JobGroup $JobGroupID
    }
  }
  $vm = Set-SCVirtualMachine -VM $vm -JobGroup $JobGroupID -RunAsynchronously

  return VMToJson $vm "Expanding"
} catch {
  ErrorToJson 'Expand VM Disks' $_
}
