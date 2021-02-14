function VMToJson($vm, $message = "") {
  $vmjson = @{}
  if ($vm.Cloud -ne $null) { $vmjson.Cloud = $vm.Cloud.Name }
  if ($vm.Name -ne $null) { $vmjson.Name = $vm.Name }
  if ($vm.Status -ne $null) { $vmjson.Status = "$($vm.Status)" }
  if ($vm.Memory -ne $null) { $vmjson.Memory = $vm.Memory }
  if ($vm.CpuCount -ne $null) { $vmjson.CpuCount = $vm.CpuCount }
  if ($vm.VirtualNetworkAdapters -ne $null) { $vmjson.VirtualNetwork = $vm.VirtualNetworkAdapters.VMNetwork.Name }
  if ($vm.BiosGuid -ne $null) { $vmjson.Guid = $vm.BiosGuid }
  if ($vm.CreationTime -ne $null) { $vmjson.CreationTime = $vm.CreationTime.ToString('o') }
  if ($vm.ModifiedTime -ne $null) { $vmjson.ModifiedTime = $vm.ModifiedTime.ToString('o') }
  if ($message) { $vmjson.Message = $message }
  $vmjson | convertto-json
}

function ErrorToJson($what,$err) {
  @{
    Message = "$($what) Failed: $($err.Exception.Message)"
    Error = "$($err) $($err.ScriptStackTrace)"
  } | convertto-json
}

function GetVM($vmname) {
  try {
    $vm = Get-SCVirtualMachine -Name $vmname
    if (-not $vm) {
      return @{ Message = "VM $($vmname) not found" } | convertto-json
    }
    return VMToJson $vm
  } catch {
    ErrorToJson 'Get VM' $_
  }
}

function CreateVM($cloud, $vmname, $vhdisk, $vmtemplate, [int]$memory, [int]$cpucount, [int]$disksize, $vmnetwork, $bootstrapdata) {
  try {
    $JobGroupID = [GUID]::NewGuid().ToString()
    if ($vhdisk) {
      $VirtualHardDisk = Get-SCVirtualHardDisk -name $vhdisk
      if (-not $VirtualHardDisk) {
        throw "VHD $($vhdisk) not found"
      }
      New-SCVirtualDiskDrive -SCSI -Bus 0 -LUN 0 -JobGroup $JobGroupID -CreateDiffDisk $false -Filename "$($vmname)_disk_1" -VolumeType BootAndSystem -VirtualHardDisk $VirtualHardDisk
      New-SCVirtualDiskDrive -SCSI -Bus 0 -LUN 1 -JobGroup $JobGroupID -VirtualHardDiskSizeMB ($disksize) -CreateDiffDisk $false -Dynamic -Filename "$($vmname)_disk_2" -VolumeType System
    } else {
      New-SCVirtualDiskDrive -SCSI -Bus 0 -LUN 0 -JobGroup $JobGroupID -VirtualHardDiskSizeMB ($disksize) -CreateDiffDisk $false -Dynamic -Filename "$($vmname)_disk_1" -VolumeType BootAndSystem
    }

    $HardwareProfile = Get-SCHardwareProfile | Where-Object {$_.Name -eq "Server Gen 2 - Medium" }
    $LinuxOS = Get-SCOperatingSystem | Where-Object {$_.name -eq 'Other Linux (64 bit)'}

    if ($vmtemplate) {
      $VMTemplateObj = Get-SCVMTemplate -Name $vmtemplate
    } else {
      $VMTemplateObj = New-SCVMTemplate -Name "Temporary Template$JobGroupID" -Generation 2 -HardwareProfile $HardwareProfile -JobGroup $JobGroupID -OperatingSystem $LinuxOS -NoCustomization
    }

    $VMNetwork = Get-SCVMNetwork -Name $vmnetwork
    $VMSubnet = $VMNetwork.VMSubnet | Select-Object -First 1

    Set-SCVirtualNetworkAdapter -JobGroup $JobGroupID -SlotID 0 -VMNetwork $VMNetwork -VMSubnet $VMSubnet
    $virtualMachineConfiguration = New-SCVMConfiguration -VMTemplate $VMTemplateObj -Name $vmname -VMHostGroup 'SO'
    $SCCloud = Get-SCCloud -Name $cloud
    $vm = New-SCVirtualMachine -Name $vmname -VMConfiguration $virtualMachineConfiguration -Cloud $SCCloud -Description "SO||talostest||manual" -JobGroup $JobGroupID -StartAction "NeverAutoTurnOnVM" -StopAction "ShutdownGuestOS" -DynamicMemoryEnabled $false -MemoryMB $memory -CPUCount $cpucount -RunAsynchronously

    return VMToJson $vm "Creating"
  } catch {
    ErrorToJson 'Create VM' $_
  }
}

function StartVM($vmname) {
  try {
    $vm = Start-SCVirtualMachine -VM $vmname -RunAsynchronously
    return VMToJson $vm "Starting"
  } catch {
    ErrorToJson 'Start VM' $_
  }
}

function StopVM($vmname) {
  try {
    $vm = Stop-SCVirtualMachine -VM $vmname -RunAsynchronously
    return VMToJson $vm "Stopping"
  } catch {
    ErrorToJson 'Stop VM' $_
  }
}

function RemoveVM($vmname) {
  try {
    $vm = Get-SCVirtualMachine $vmname
    if (-not $vm) {
      return (@{ Message = "Removed" } | convertto-json)
    }
    if ($vm.Status -eq 'PowerOff') {
      $vm = Remove-SCVirtualMachine $vm -RunAsynchronously
      VMToJson $vm "Removing"
    } else {
      $vm = Stop-SCVirtualmachine $vm -RunAsynchronously
      VMToJson $vm "Stopping"
    }
  } catch {
    ErrorToJson 'Remove VM' $_
  }
}

