param($vm, $disk, $lun, $generation, $JobGroup)

if ($lun -gt 16) {
  throw "Too many virtual disks"
}
$vhdargs = @{
  Bus = 0
  LUN = $lun
  CreateDiffDisk = $false
  Filename = "$($vmname)_disk_$($lun + 1)"
}
if ($JobGroup) {
  $vhdargs.JobGroup = $JobGroup
}
if ($vm) {
  $vhdargs.VM = $vm
  $generation = $vm.Generation
}
if ($lun -eq 0) {
  $vhdargs['VolumeType'] = 'BootAndSystem'
  if ($generation -ge 2) {
    $vhdargs['SCSI'] = $true
  } else {
    $vhdargs['IDE'] = $true
  }
} else {
  $vhdargs['SCSI'] = $true
  if ($generation -ge 2) {
    $vhdargs['VolumeType'] = 'System'
  } else {
    $vhdargs['VolumeType'] = 'None'
  }
}
if ($disk.volumeType) {
  $vhdargs['VolumeType'] = $disk.volumeType
}
if ($disk.storageQoSPolicy) {
  $vhdargs['StorageQoSPolicy'] = (Get-SCStorageQoSPolicy -Name $disk.storageQoSPolicy | Select-Object -First 1)
  if (-not $vhdargs['StorageQoSPolicy']) {
    throw "StorageQoSPolicy $($disk.storageQoSPolicy) not found"
  }
}
if ($disk.filename) {
  $vhdargs.Filename = $disk.filename
}
if ($disk.path) {
  $vhdargs.Path = $disk.path
}
if ($disk.vmHost) {
  $vhdargs['UseLocalVirtualHardDisk'] = $true
} elseif ($disk.vhDisk) {
  $vhdargs['VirtualHardDisk'] = (Get-SCVirtualHardDisk -name $disk.vhDisk | Select-Object -First 1)
  if (-not $vhdargs['VirtualHardDisk']) {
    throw "VHD $($disk.vhDisk) not found"
  }
} else {
  if ($disk.dynamic) {
    $vhdargs['Dynamic'] = $true
  } else {
    $vhdargs['Fixed'] = $true
  }
  $vhdargs.VirtualHardDiskSizeMB = $disk.sizeMB
}
if ($vhdargs.VirtualHardDiskSizeMB -or $vhdargs.UseLocalVirtualHardDisk -or $vhdargs.VirtualHardDisk) {
  New-SCVirtualDiskDrive @vhdargs
}
