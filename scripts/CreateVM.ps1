param($cloud, $hostgroup, $vmname, $vmtemplate, [int]$memory, [int]$memorymin, [int]$memorymax, [int]$memorybuffer, [int]$cpucount, $disks, $networkdevices, $fibrechannel, $hardwareprofile, $operatingsystem, $availabilityset, $vmoptions)
try {
  $JobGroupID = [GUID]::NewGuid().ToString()
  $disklist = $disks | ConvertFrom-Json

  if ($vmtemplate) {
    $VMTemplateObj = Get-SCVMTemplate -Name $vmtemplate
    $generation = $VMTemplateObj.Generation

    for ($lun = 0; $lun -lt $disklist.Length; $lun++) {
      if (-not $disklist[$lun].vmHost) {
        CreateVHD -disk $disklist[$lun] -lun $lun -generation $generation -JobGroup $JobGroupID
      }
    }
  } else {
    $HardwareProfile = Get-SCHardwareProfile | Where-Object Name -eq $hardwareprofile
    if (-not $operatingsystem) {
      foreach ($disk in $disklist) {
	if ($disk.vhDisk) {
	  $LinuxOS = Get-SCVirtualHardDisk -name $disk.vhDisk | Select-Object -First 1 -Expand OperatingSystem
	}
      }
      if (-not $LinuxOS) {
	$LinuxOS = Get-SCOperatingSystem | Where-Object Name -eq 'Other Linux (64 bit)'
      }
    } else {
      $LinuxOS = Get-SCOperatingSystem | Where-Object Name -eq $operatingsystem
    }
    $generation = $HardwareProfile.generation

    for ($lun = 0; $lun -lt $disklist.Length; $lun++) {
      if (-not $disklist[$lun].vmHost) {
        CreateVHD -disk $disklist[$lun] -lun $lun -generation $generation -JobGroup $JobGroupID
      }
    }

    $VMTemplateObj = New-SCVMTemplate -Name "Temporary Template $JobGroupID" -Generation $generation -HardwareProfile $HardwareProfile -JobGroup $JobGroupID -OperatingSystem $LinuxOS -NoCustomization -ErrorAction Stop
  }

  $hostclusters = @()
  $hgtodo = @(Get-SCVMHostGroup -Name $hostgroup)
  while ($hgtodo.Count -gt 0) {
    foreach ($hg in $hgtodo) {
      $hostclusters += Get-SCVMHostCluster -VMHostGroup $hg
    }
    $hgtodo = foreach ($hg in $hgtodo) {
      Get-SCVMHostGroup -ParentHostGroup $hg
    }
  }
  $networkslot = 0
  foreach ($networkdevice in ($networkdevices | ConvertFrom-Json)) {
    $VMNetwork = foreach ($hc in $hostclusters) {
      Get-SCVirtualNetwork -VMHostCluster $hc | Select-Object -Expand LogicalNetworks | ForEach-Object { Get-SCVMNetwork -LogicalNetwork $_ -Name $networkdevice.VMNetwork }
    }
    $VMNetwork = $VMNetwork | Sort-Object -Unique
    if ($VMNetwork.Count -lt 1) {
      throw "No vmnetwork found with name $($networkdevice.VMNetwork) found in hostgroup $hostgroup"
    }
    if ($VMNetwork.Count -gt 1) {
      throw "More than one vmnetwork found with name $($networkdevice.VMNetwork) found in hostgroup $hostgroup ($($VMNetwork.Name))"
    }
    $VMSubnet = $VMNetwork.VMSubnet | Select-Object -First 1

    $vnargs = @{
      SlotID = $networkslot
      VMNetwork = $VMNetwork
      VMSubnet = $VMSubnet
    }
    if ($networkslot -lt $VMTemplateObj.VirtualNetworkAdapters.Count) {
      Set-SCVirtualNetworkAdapter -JobGroup $JobGroupID -SlotID $networkslot -VMNetwork $VMNetwork 
    } else {
      New-SCVirtualNetworkAdapter -JobGroup $JobGroupID @vnargs
    }
    $networkslot = $networkslot + 1
  }
  foreach ($fc in ($fibrechannel | ConvertFrom-Json)) {
    $fcargs = @{
      VMTemplate = $VMTemplateObj
    }
    if ($fc.StorageFabricClassification) {
      $fcsc = Get-SCStorageFabricClassification -Name $fc.StorageFabricClassification
      if (-not $fcsc) {
        throw "Storage Fabric Classification $($fc.StorageFabricClassification) not found"
      }
      $fcargs['StorageFabricClassification'] = $fcsc
    }
    if ($fc.VirtualSAN) {
      $fcvsan = Get-SCVMHostFibreChannelVirtualSAN -Name $fc.VirtualSAN
      if (-not $fcvsan) {
        throw "Virtual SAN $($fc.VirtualSAN) not found"
      }
      $fcargs['VirtualFibreChannelSAN'] = $fcvsan
    }
    New-SCVirtualFibreChannelAdapter @fcargs | Out-Null
  }

  $vmargs = @{
    Name = "$vmname"
    CPUCount = $cpucount
    DynamicMemoryEnabled = $false
  }
  $optionsobject = $vmoptions | ConvertFrom-Json
  foreach ($optkey in @('Description','StartAction','StopAction','CPULimitForMigration','CPULimitFunctionality','EnableNestedVirtualization','CheckpointType')) {
      if ($optionsobject.$optkey -ne $null) { $vmargs[$optkey] = $optionsobject.$optkey }
  }
  if ($availabilityset) {
    $vmargs.VMConfiguration = New-SCVMConfiguration -VMTemplate $VMTemplateObj -Name $vmname -VMHostGroup $hostgroup -AvailabilitySetNames $availabilityset
  } else {
    $vmargs.VMConfiguration = New-SCVMConfiguration -VMTemplate $VMTemplateObj -Name $vmname -VMHostGroup $hostgroup
  }
  $vmargs.Cloud = Get-SCCloud -Name $cloud

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
  if (-not $vmtemplate -and $VMTemplateObj) {
    try {
      Remove-SCVMTemplate $VMTemplateObj | out-null
    } catch {
    }
  }
  ErrorToJson 'Create VM' $_
}

