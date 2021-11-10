$ProgressPreference = 'SilentlyContinue'
$WarningPreference = 'SilentlyContinue'
$VerbosePreference = 'SilentlyContinue'
$InformationPreference = 'SilentlyContinue'
$DebugPreference = 'SilentlyContinue'

function VMToJson($vm, $message = "") {
  $vmjson = @{}
  if ($vm.Cloud -ne $null) { $vmjson.Cloud = $vm.Cloud.Name }
  if ($vm.Name -ne $null) { $vmjson.Name = $vm.Name }
  if ($vm.Status -ne $null) { $vmjson.Status = "$($vm.Status)" }
  if ($vm.Memory -ne $null) { $vmjson.Memory = $vm.Memory }
  if ($vm.CpuCount -ne $null) { $vmjson.CpuCount = $vm.CpuCount }
  if ($vm.VirtualNetworkAdapters -ne $null) {
    $vmjson.VirtualNetwork = $vm.VirtualNetworkAdapters.VMNetwork.Name | select -first 1
    if ($vm.VirtualNetworkAdapters.IPv4Addresses) {
      $vmjson.IPv4Addresses = @($vm.VirtualNetworkAdapters.IPv4Addresses)
    }
    if ($vm.VirtualNetworkAdapters.Name) {
      $vmjson.Hostname = $vm.VirtualNetworkAdapters.Name | select -first 1
    }
  }
  if ($vm.VirtualHardDisks -ne $null) {
    $vmjson.VirtualDisks = @($vm.VirtualHardDisks | select Size, MaximumSize)
  }
  if ($vm.BiosGuid -ne $null) { $vmjson.BiosGuid = "$($vm.BiosGuid)" }
  if ($vm.Id -ne $null) { $vmjson.Id = "$($vm.Id)" }
  if ($vm.CreationTime -ne $null) { $vmjson.CreationTime = $vm.CreationTime.ToString('o') }
  if ($vm.ModifiedTime -ne $null) { $vmjson.ModifiedTime = $vm.ModifiedTime.ToString('o') }
  if ($message) { $vmjson.Message = $message }
  $vmjson | convertto-json -Depth 2 -Compress
}

function ErrorToJson($what,$err) {
  @{
    Message = "$($what) Failed: $($err.Exception.Message)"
    Error = "$($err) $($err.ScriptStackTrace)"
  } | convertto-json -Depth 2 -Compress
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

function ReadVM($vmname) {
  try {
    $vm = Get-SCVirtualMachine -Name $vmname
    if (-not $vm) {
      return @{ Message = "VM $($vmname) not found" } | convertto-json
    }
    Read-SCVirtualMachine -vm $vm -RunAsynchronously | out-null
    return VMToJson $vm
  } catch {
    ErrorToJson 'Get VM' $_
  }
}

function CreateVM($cloud, $hostgroup, $vmname, $vmtemplate, [int]$memory, [int]$cpucount, $disks, $vmnetwork, $hardwareprofile, $description, $startaction, $stopaction) {
  try {
    if (-not $description) { $description = "$hostgroup||capi-scvmm" }
    if (-not $startaction) { $startaction = 'NeverAutoTurnOnVM' }
    if (-not $stopaction) { $stopaction = 'ShutdownGuestOS' }
    $JobGroupID = [GUID]::NewGuid().ToString()
    $disknum = 0
    $voltype = 'BootAndSystem'
    foreach ($disk in ($disks | convertfrom-json)) {
      if ($disknum -gt 16) {
        throw "Too many virtual disks"
      }
      if ($disk.vhDisk) {
        $VirtualHardDisk = Get-SCVirtualHardDisk -name $disk.vhDisk
        if (-not $VirtualHardDisk) {
          throw "VHD $($disk.vhDisk) not found"
        }
        New-SCVirtualDiskDrive -SCSI -Bus 0 -LUN $disknum -JobGroup $JobGroupID -CreateDiffDisk $false -Filename "$($vmname)_disk_1" -VolumeType $voltype -VirtualHardDisk $VirtualHardDisk
      } else {
        New-SCVirtualDiskDrive -SCSI -Bus 0 -LUN $disknum -JobGroup $JobGroupID -VirtualHardDiskSizeMB ($disk.sizeMB) -CreateDiffDisk $false -Dynamic:$($disk.dynamic) -Filename "$($vmname)_disk_$($disknum + 1)" -VolumeType $voltype
      }
      $disknum = $disknum + 1
      $voltype = 'System'
    }

    if ($vmtemplate) {
      $VMTemplateObj = Get-SCVMTemplate -Name $vmtemplate
    } else {
      $HardwareProfile = Get-SCHardwareProfile | Where-Object {$_.Name -eq $hardwareprofile }
      $LinuxOS = Get-SCOperatingSystem | Where-Object {$_.name -eq 'Other Linux (64 bit)'}
      $generation = $HardwareProfile.generation

      $VMTemplateObj = New-SCVMTemplate -Name "Temporary Template$JobGroupID" -Generation $generation -HardwareProfile $HardwareProfile -JobGroup $JobGroupID -OperatingSystem $LinuxOS -NoCustomization
    }

    $VMNetwork = Get-SCVMNetwork -Name $vmnetwork
    $VMSubnet = $VMNetwork.VMSubnet | Select-Object -First 1

    Set-SCVirtualNetworkAdapter -JobGroup $JobGroupID -SlotID 0 -VMNetwork $VMNetwork -VMSubnet $VMSubnet
    $virtualMachineConfiguration = New-SCVMConfiguration -VMTemplate $VMTemplateObj -Name $vmname -VMHostGroup $hostgroup
    $SCCloud = Get-SCCloud -Name $cloud
    $vm = New-SCVirtualMachine -Name $vmname -VMConfiguration $virtualMachineConfiguration -Cloud $SCCloud -Description $description -JobGroup $JobGroupID -StartAction $startaction -StopAction $stopaction -DynamicMemoryEnabled $false -MemoryMB $memory -CPUCount $cpucount -RunAsynchronously

    return VMToJson $vm "Creating"
  } catch {
    ErrorToJson 'Create VM' $_
  }
}

function ExpandVMDisks($vmname, $disks) {
  try {
    $vm = Get-SCVirtualMachine -Name $vmname
    if (-not $vm) {
      throw "Virtual Machine $vmname not found"
    }
    foreach ($vhdisk in Get-SCVirtualDiskDrive -vm $vm) {
      $lun = $vhdisk.LUN
      if ($disks[$lun].sizeMB) {
        Expand-SCVirtualDiskDrive -VirtualDiskDrive $vhdisk -VirtualHardDiskSizeGB ($disks[$lun].sizeMB / 1024) -JobGroup $JobGroupID -RunAsynchronously
      }
    }
    return VMToJson $vm "Resizing"
  } catch {
    ErrorToJson 'Expand VM Disks' $_
  }
}

function AddIsoToVM($vmname, $isopath) {
  try {
    $shr = Get-SCLibraryShare | ?{ $isopath.StartsWith($_.Path) } | select -first 1
    if (-not $shr) {
      throw "Library share containing $isopath not found"
    }
    $pdir = split-path ($isopath.Remove(0,$shr.Path.length+1))
    Read-SCLibraryShare -LibraryShare $shr -Path $pdir | out-null
    $ISO = Get-SCISO | Where-Object { $_.SharePath -eq $isopath } | select -first 1
    if (-not $ISO) {
      throw "Isofile $isopath not found"
    }
    $vm = Get-SCVirtualMachine -Name $vmname
    if (-not $vm) {
      throw "Virtual Machine $vmname not found"
    }
    $DVDDrive = Get-SCVirtualDVDDrive -VM $vm | select -first 1
    Set-SCVirtualDVDDrive -VirtualDVDDrive $DVDDrive -ISO $ISO -Link | out-null

    $vm = Start-SCVirtualMachine -VM $vm -RunAsynchronously
    return VMToJson $vm "Starting"
  } catch {
    ErrorToJson 'Add ISO to VM' $_
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

function GenerateVMName($spec, $metadata) {
  try {
    $specobj = $spec | convertfrom-json
    $metadataobj = $metadata | convertfrom-json
    $newspec = @{}
    if (-not $specobj.VMName) {
      $namerange = $metadataobj.annotations.'scvmmmachine.cluster.x-k8s.io/vmnames'
      if ($namerange) {
        $rstart, $rend = $namerange -split ':'
        $vmname = $rstart
        while ($vmname -le $rend) {
          if (-not (Get-SCVirtualMachine -Name $vmname)) { break }
          $nextname = ""
          $brk = $false
          for ($vi = $vmname.length-1; $vi -ge 0; $vi = $vi-1) {
            $chr = $vmname[$vi]
            if (-not $brk) {
              if ($chr -match '[0-8A-Ya-y]') {
                $chr = [char]([int]$chr+1)
                $brk = $true
              } elseif ($chr -eq '9') {
                $chr = '0'
              } else {
                $chr=[char]([int]$chr-25)
              }
            }
            $nextname = "$chr$nextname"
          }
          $vmname = $nextname
        }
        if ($vmname -le $rend) {
          $newspec.VMName = $vmname
        } else {
          throw "no vmname available in range $namerange"
        }
      } else {
        $newspec.VMName = $metadataobj.name
      }
    }
    return $newspec | convertto-json -depth 3 -compress
  } catch {
    ErrorToJson 'Generate VM Name' $_
  }
}

function CreateADComputer($name, $oupath, $domaincontroller, $description, $memberof) {
  try {
    $dcparam = @{}
    if ($domaincontroller) {
      $dcparam.Server = $domaincontroller
    }
    $ident = "CN=$($name),$($oupath)"
    try {
      $comp = Get-ADComputer @credparam @dcparam -Identity $ident
      if (-not $comp) {
        throw "ADComputer $ident not found"
      }
    } catch {
      New-ADComputer @credparam @dcparam -Name $name -Path $oupath -AccountPassword $null -samaccountname "$($name)`$" -enabled $true -Description $description
    }
    foreach ($grp in $memberof) {
      Add-ADGroupMember @credparam @dcparam -Identity $grp -Members $ident
    }
    return @{ Message = "ADComputer $($ident) created" } | convertto-json
  } catch {
    ErrorToJson 'Create AD Computer' $_
  }
}

function RemoveADComputer($name, $oupath, $domaincontroller) {
  try {
    $dcparam = @{}
    if ($domaincontroller) {
      $dcparam.Server = $domaincontroller
    }
    $ident = "CN=$($name),$($oupath)"
    try {
      $comp = Get-ADComputer @credparam @dcparam -Identity $ident
    } catch {
      return @{ Message = "ADComputer $($ident) not found" } | convertto-json
    }
    Remove-ADComputer @credparam @dcparam -Confirm:$false -Identity $ident
    return @{ Message = "ADComputer $($ident) removed" } | convertto-json
  } catch {
    ErrorToJson 'Remove AD Computer' $_
  }
}
