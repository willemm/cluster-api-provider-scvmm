param($cloud, $hostgroup, $vmname, $vmtemplate, [int]$memory, [int]$memorymin, [int]$memorymax, [int]$memorybuffer, [int]$cpucount, $disks, $vmnetwork, $hardwareprofile, $description, $startaction, $stopaction, $cpulimitformigration, $cpulimitfunctionality, $operatingsystem, $domain)
try {
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
    if (-not $operatingsystem) { $operatingsystem = 'Other Linux (64 bit)' }
    $LinuxOS = Get-SCOperatingSystem | Where-Object {$_.name -eq $operatingsystem }
    $generation = $HardwareProfile.generation

    $VMTemplateObj = New-SCVMTemplate -Name "Temporary Template$JobGroupID" -Generation $generation -HardwareProfile $HardwareProfile -JobGroup $JobGroupID -OperatingSystem $LinuxOS -NoCustomization -ErrorAction Stop
  }

  $VMNetwork = Get-SCVMNetwork -Name $vmnetwork
  $VMSubnet = $VMNetwork.VMSubnet | Select-Object -First 1

  Set-SCVirtualNetworkAdapter -JobGroup $JobGroupID -SlotID 0 -VMNetwork $VMNetwork -VMSubnet $VMSubnet

  $vmargs = @{
    Name = "$vmname"
    CPUCount = $cpucount
    DynamicMemoryEnabled = $false
  }
  $vmargs.VMConfiguration = New-SCVMConfiguration -VMTemplate $VMTemplateObj -Name $vmname -VMHostGroup $hostgroup
  $vmargs.Cloud = Get-SCCloud -Name $cloud

  if ($description) { $vmargs.Description = "$description" }
  if ($startaction) { $vmargs.StartAction = "$startaction" }
  if ($stopaction) { $vmargs.StopAction = "$stopaction" }
  if ($replicationgroup) { $vmargs.ReplicationGroup = "$replicationgroup" }
  $bl = $false
  if ([bool]::TryParse($cpulimitformigration, [ref]$bl)) { $vmargs.CPULimitForMigration = $bl }
  if ([bool]::TryParse($cpulimitfunctionality, [ref]$bl)) { $vmargs.CPULimitFunctionality = $bl }
  if ($domain) {
    if (${env:DOMAIN_USERNAME} -and ${env:DOMAIN_PASSWORD}) {
      $vmargs.Domain = $domain
      $vmargs.DomainJoinCredential = new-object PSCredential(${env:DOMAIN_USERNAME}, (ConvertTo-Securestring -force -AsPlainText -String ${env:DOMAIN_PASSWORD}))
    }
  }

  if ($memory -gt 0) { $vmargs.MemoryMB = $memory }
  if ($memorymin -ge 0) {
    $vmargs.DynamicMemoryMin = $memorymin
    $vmargs.DynamicMemoryEnabled = $true
  }
  if ($memorymin -ge 0) {
    $vmargs.DynamicMemoryMax = $memorymax
    $vmargs.DynamicMemoryEnabled = $true
  }
  if ($memorybuffer -ge 0) {
    $vmargs.DynamicMemoryBuffer = $memorybuffer
  }
  $vm = New-SCVirtualMachine @vmargs -JobGroup $JobGroupID -RunAsynchronously -ErrorAction Stop

  return VMToJson $vm "Creating"
} catch {
  ErrorToJson 'Create VM' $_
}
