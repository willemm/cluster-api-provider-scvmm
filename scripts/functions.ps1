function VMToJson($vm, $message = "") {
  @{
    Cloud = $vm.Cloud.Name
    Name = $vm.Name
    Status = "$($vm.Status)"
    Memory = $vm.Memory
    CpuCount = $vm.CpuCount
    VirtualNetwork = $vm.VirtualNetworkAdapters.VMNetwork.Name
    Guid = $vm.BiosGuid
    CreationTime = $vm.CreationTime.ToString('o')
    ModifiedTime = $vm.ModifiedTime.ToString('o')
    Message = $message
  } | convertto-json
}

function ErrorToJson($what,$err) {
  @{
    Message = "$($what) Failed: $($err.Exception.Message)"
    Error = "$($err.Exception)"
  } | convertto-json
}

function GetVM($vmname) {
  try {
    $vm = Get-SCVirtualMachine -Name $vmname
    if (-not $vm) {
      return @{ Message = "VM $($vmname) not found" } | convertto-json
    }
    return VMToJson($vm)
  } catch {
    ErrorToJson('Get VM', $_)
  }
}

function CreateVM($cloud, $vmname, [int]$memory, [int]$cpucount, [int]$disksize, $vmnetwork, $bootstrapdata) {
  $JobGroupID = [GUID]::NewGuid().ToString()
  New-SCVirtualDiskDrive -SCSI -Bus 0 -LUN 0 -JobGroup $JobGroupID -VirtualHardDiskSizeMB ($disksize) -CreateDiffDisk $false -Dynamic -Filename "$($vmname)_disk_1" -VolumeType BootAndSystem

  $HardwareProfile = Get-SCHardwareProfile | Where-Object {$_.Name -eq "Server Gen 2 - Medium" }
  $LinuxOS = Get-SCOperatingSystem | Where-Object {$_.name -eq 'Other Linux (64 bit)'}

  $VMTemplate = New-SCVMTemplate -Name "Temporary Template$JobGroupID" -Generation 2 -HardwareProfile $HardwareProfile -JobGroup $JobGroupID -OperatingSystem $LinuxOS -NoCustomization

  $VMNetwork = Get-SCVMNetwork -Name $vmnetwork
  $VMSubnet = $VMNetwork.VMSubnet | Select-Object -First 1

  Set-SCVirtualNetworkAdapter -JobGroup $JobGroupID -SlotID 0 -VMNetwork $VMNetwork -VMSubnet $VMSubnet
  $virtualMachineConfiguration = New-SCVMConfiguration -VMTemplate $VMTemplate -Name $vmname -VMHostGroup 'SO'
  $SCCloud = Get-SCCloud -Name $cloud
  New-SCVirtualMachine -Name $vmname -VMConfiguration $virtualMachineConfiguration -Cloud $SCCloud -Description "SO||talostest||manual" -JobGroup $JobGroupID -StartAction "NeverAutoTurnOnVM" -StopAction "ShutdownGuestOS" -DynamicMemoryEnabled $false -MemoryMB $memory -CPUCount $cpucount -ReturnImmediately

  Get-SCVirtualMachine -name $vmname
}

function StartVM($vmname) {
  try {
    $vm = Start-SCVirtualMachine -VM $vmname
    return VMToJson($vm, "Starting")
  } catch {
    ErrorToJson('Start VM', $_)
  }
}

function Stop($vmname) {
  try {
    $vm = Stop-SCVirtualMachine -VM $vmname
    return VMToJson($vm, "Stopping")
  } catch {
    ErrorToJson('Stop VM', $_)
  }
}

function RemoveVM($vmname) {
  try {
    $vm = Get-SCVirtualMachine $vmname
    if (-not $vm) {
      return (@{ Message = "Removed" } | convertto-json)
    }
    if ($vm.Status -eq 'PowerOff') {
      $vm = Remove-SCVirtualMachine $vmname
      VMToJson($vm, "Removing")
    } else {
      $vm = Stop-SCVirtualmachine $vm
      VMToJson($vm, "Stopping")
    }
  } catch {
    ErrorToJson('Remove VM', $_)
  }
}

