param($cloud, $hostgroup, $vmname, $vmtemplate, [int]$memory, [int]$memorymin, [int]$memorymax, [int]$memorybuffer, [int]$cpucount, $disks, $vmnetwork, $hardwareprofile, $description, $startaction, $stopaction, $cpulimitformigration, $cpulimitfunctionality, $operatingsystem, $domain, $availabilityset)
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
  foreach ($disk in ($disks | convertfrom-json)) {
    if ($disknum -gt 16) {
      throw "Too many virtual disks"
    }
    $vhdargs = @{
      Bus = 0
      LUN = $disknum
      JobGroup = $JobGroupID
      CreateDiffDisk = $false
      Filename = "$($vmname)_disk_$($disknum + 1)"
    }
    if ($disknum -eq 0) {
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
    if ($disk.vhDisk) {
      $vhdargs['VirtualHardDisk'] = Get-SCVirtualHardDisk -name $disk.vhDisk
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
    New-SCVirtualDiskDrive @vhdargs
    $disknum = $disknum + 1
  }

  if ($vmtemplate) {
    $VMTemplateObj = Get-SCVMTemplate -Name $vmtemplate
  } else {
    $HardwareProfile = Get-SCHardwareProfile | Where-Object {$_.Name -eq $hardwareprofile }
    if (-not $operatingsystem -and $VirtualHardDisk -and $VirtualHardDisk.OperatingSystem) {
      $LinuxOS = $VirtualHardDisk.OperatingSystem
    } else {
      if (-not $operatingsystem) {
        $operatingsystem = 'Other Linux (64 bit)'
      }
      $LinuxOS = Get-SCOperatingSystem | Where-Object {$_.name -eq $operatingsystem }
    }
    $generation = $HardwareProfile.generation

    $VMTemplateObj = New-SCVMTemplate -Name "Temporary Template $JobGroupID" -Generation $generation -HardwareProfile $HardwareProfile -JobGroup $JobGroupID -OperatingSystem $LinuxOS -NoCustomization -ErrorAction Stop
  }

  $VMNetwork = Get-SCVMNetwork -Name $vmnetwork
  $VMSubnet = $VMNetwork.VMSubnet | Select-Object -First 1

  Set-SCVirtualNetworkAdapter -JobGroup $JobGroupID -SlotID 0 -VMNetwork $VMNetwork -VMSubnet $VMSubnet

  $vmargs = @{
    Name = "$vmname"
    CPUCount = $cpucount
    DynamicMemoryEnabled = $false
  }
  if ($availabilityset) {
    $vmargs.VMConfiguration = New-SCVMConfiguration -VMTemplate $VMTemplateObj -Name $vmname -VMHostGroup $hostgroup -AvailabilitySetNames $availabilityset
  } else {
    $vmargs.VMConfiguration = New-SCVMConfiguration -VMTemplate $VMTemplateObj -Name $vmname -VMHostGroup $hostgroup
  }
  $vmargs.Cloud = Get-SCCloud -Name $cloud

  if ($description) { $vmargs.Description = "$description" }
  if ($startaction) { $vmargs.StartAction = "$startaction" }
  if ($stopaction) { $vmargs.StopAction = "$stopaction" }
  $bl = $false
  if ([bool]::TryParse($cpulimitformigration, [ref]$bl)) { $vmargs.CPULimitForMigration = $bl }
  if ([bool]::TryParse($cpulimitfunctionality, [ref]$bl)) { $vmargs.CPULimitFunctionality = $bl }
  if ($domain) {
    if (${env:DOMAIN_USERNAME} -and ${env:DOMAIN_PASSWORD}) {
      $vmargs.Domain = $domain
      $vmargs.DomainJoinCredential = new-object PSCredential(${env:DOMAIN_USERNAME}, (ConvertTo-Securestring -force -AsPlainText -String ${env:DOMAIN_PASSWORD}))
    }
  }

  if ($memorymin -ge 0) {
    $vmargs.DynamicMemoryMin = $memorymin
    $vmargs.DynamicMemoryEnabled = $true
    $vmargs.MemoryMB = $memorymin
  }
  if ($memorymax -ge 0) {
    $vmargs.DynamicMemoryMax = $memorymax
    $vmargs.DynamicMemoryEnabled = $true
  }
  if ($memorybuffer -ge 0) {
    $vmargs.DynamicMemoryBuffer = $memorybuffer
  }
  if ($memory -gt 0) { $vmargs.MemoryMB = $memory }
  $vm = New-SCVirtualMachine @vmargs -JobGroup $JobGroupID -RunAsynchronously -ErrorAction Stop
  if ($vm.Status -eq 'CreationFailed') {
    $msg = "Unknown error"
    if ($vm.MostRecentTaskIfLocal.ErrorInfo) {
      $msg = $vm.MostRecentTaskIfLocal.ErrorInfo
    }
    throw "Creation Failed: $msg"
  }

  return VMToJson $vm "Creating"
} catch {
  ErrorToJson 'Create VM' $_
}

