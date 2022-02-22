param($cloud, $hostgroup, $vmname, $vmtemplate, [int]$memory, [int]$cpucount, $disks, $vmnetwork, $hardwareprofile, $description, $startaction, $stopaction)
try {
  if (-not $description) { $description = "$hostgroup||capi-scvmm" }
  if (-not $startaction) { $startaction = 'NeverAutoTurnOnVM' }
  if (-not $stopaction) { $stopaction = 'ShutdownGuestOS' }

  $generation = 1
  if ($vmtemplate) {
    $VMTemplateObj = Get-SCVMTemplate -Name $vmtemplate
    $generation = $VMTemplateObj.Generation
  } else {
    $HardwareProfile = Get-SCHardwareProfile | Where-Object {$_.Name -eq $hardwareprofile }
    $generation = $HardwareProfile.generation
  }

  $JobGroupID = [GUID]::NewGuid().ToString()
  $disknum = 0
  $voltype = 'BootAndSystem'
  $scsi = $generation -ge 2
  foreach ($disk in ($disks | convertfrom-json)) {
    if ($disknum -gt 16) {
      throw "Too many virtual disks"
    }
    if ($disk.vhDisk) {
      $VirtualHardDisk = Get-SCVirtualHardDisk -name $disk.vhDisk
      if (-not $VirtualHardDisk) {
        throw "VHD $($disk.vhDisk) not found"
      }
      if ($scsi) {
        New-SCVirtualDiskDrive -SCSI -Bus 0 -LUN $disknum -JobGroup $JobGroupID -CreateDiffDisk $false -Filename "$($vmname)_disk_1" -VolumeType $voltype -VirtualHardDisk $VirtualHardDisk
      } else {
        New-SCVirtualDiskDrive -IDE -Bus 0 -LUN $disknum -JobGroup $JobGroupID -CreateDiffDisk $false -Filename "$($vmname)_disk_1" -VolumeType $voltype -VirtualHardDisk $VirtualHardDisk
      }
    } else {
      if ($scsi) {
        New-SCVirtualDiskDrive -SCSI -Bus 0 -LUN $disknum -JobGroup $JobGroupID -VirtualHardDiskSizeMB ($disk.sizeMB) -CreateDiffDisk $false -Dynamic:$($disk.dynamic) -Filename "$($vmname)_disk_$($disknum + 1)" -VolumeType $voltype
      } else {
        New-SCVirtualDiskDrive -IDE -Bus 0 -LUN $disknum -JobGroup $JobGroupID -VirtualHardDiskSizeMB ($disk.sizeMB) -CreateDiffDisk $false -Dynamic:$($disk.dynamic) -Filename "$($vmname)_disk_$($disknum + 1)" -VolumeType $voltype
      }
    }
    $disknum = $disknum + 1
    if ($generation -ge 2) {
      $voltype = 'System'
    } else {
      $voltype = 'None'
      $scsi = $true
    }
  }

  if ($vmtemplate) {
    $VMTemplateObj = Get-SCVMTemplate -Name $vmtemplate
  } else {
    $HardwareProfile = Get-SCHardwareProfile | Where-Object {$_.Name -eq $hardwareprofile }
    $LinuxOS = Get-SCOperatingSystem | Where-Object {$_.name -eq 'Other Linux (64 bit)'}
    $generation = $HardwareProfile.generation

    $VMTemplateObj = New-SCVMTemplate -Name "Temporary Template$JobGroupID" -Generation $generation -HardwareProfile $HardwareProfile -JobGroup $JobGroupID -OperatingSystem $LinuxOS -NoCustomization -ErrorAction Stop
  }

  $VMNetwork = Get-SCVMNetwork -Name $vmnetwork
  $VMSubnet = $VMNetwork.VMSubnet | Select-Object -First 1

  Set-SCVirtualNetworkAdapter -JobGroup $JobGroupID -SlotID 0 -VMNetwork $VMNetwork -VMSubnet $VMSubnet
  $virtualMachineConfiguration = New-SCVMConfiguration -VMTemplate $VMTemplateObj -Name $vmname -VMHostGroup $hostgroup
  $SCCloud = Get-SCCloud -Name $cloud
  $vm = New-SCVirtualMachine -Name $vmname -VMConfiguration $virtualMachineConfiguration -Cloud $SCCloud -Description $description -JobGroup $JobGroupID -StartAction $startaction -StopAction $stopaction -DynamicMemoryEnabled $false -MemoryMB $memory -CPUCount $cpucount -RunAsynchronously -ErrorAction Stop

  return VMToJson $vm "Creating"
} catch {
  ErrorToJson 'Create VM' $_
}
