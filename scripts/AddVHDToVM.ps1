param($id, $cipath, $devicetype)
try {
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    throw "Virtual Machine with ID $id not found"
  }
  if ($vm.MostRecentTask -and $vm.MostRecentTask.Status -notin 'Completed','SucceedWithInfo') {
    return VMToJson $vm "Machine is busy"
  }
  $evhd = if ($devicetype -eq "ide") {
    $eloc = "IDE:1:0"
    $vm.VirtualDiskDrives | Where-Object {
      $_.BusType -eq 'IDE' -and $_.Bus -eq 1 -and $_.Lun -eq 0
    }
  } else {
    $eloc = "IDE:0:62"
    $vm.VirtualDiskDrives | Where-Object {
      $_.BusType -eq 'SCSI' -and $_.Bus -eq 0 -and $_.Lun -eq 62
    }
  }
  if ($evhd) {
    throw "Some VHD is already attached on $eloc ($($evhd.VirtualHardDisk.Name))"
  }
  $shr = Get-SCLibraryShare | ?{ $cipath.StartsWith($_.Path) } | select -first 1
  if (-not $shr) {
    throw "Library share containing $cipath not found"
  }
  $pdir = split-path ($cipath.Remove(0,$shr.Path.length+1))
  Read-SCLibraryShare -LibraryShare $shr -Path $pdir | out-null
  $vhd = Get-SCVirtualHardDisk | Where-Object { $_.SharePath -eq $cipath } | select -first 1
  if (-not $vhd) {
    throw "VirtualHardDisk $cipath not found"
  }
  if ($devicetype -eq "ide") {
      New-SCVirtualDiskDrive -VM $vm -VirtualHardDisk $vhd -IDE -BUS 1 -LUN 0 | Out-Null
  } else {
      New-SCVirtualDiskDrive -VM $vm -VirtualHardDisk $vhd -SCSI -BUS 0 -LUN 62 | Out-Null
  }

  Remove-SCVirtualHardDisk -VirtualHardDisk $vhd -RunAsynchronously | out-null
  return VMToJson $vm "AddingVHD"
} catch {
  ErrorToJson 'Add VHD to VM' $_
}
