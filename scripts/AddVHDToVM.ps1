param($id, $cipath, $devicetype)
try {
  $shr = Get-SCLibraryShare | ?{ $cipath.StartsWith($_.Path) } | select -first 1
  if (-not $shr) {
    throw "Library share containing $cipath not found"
  }
  $pdir = split-path ($cipath.Remove(0,$shr.Path.length+1))
  Read-SCLibraryShare -LibraryShare $shr -Path $pdir | out-null
  $vhd = Get-SCVirtualHardDisk | Where-Object { $_.SharePath -eq $cipath } | select -first 1
  if (-not $vhd) {
    throw "Firtualfloppydisk $cipath not found"
  }
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    throw "Virtual Machine with ID $id not found"
  }
  if ($devicetype -eq "ide") {
      New-SCVirtualDiskDrive -VM $vm -VirtualHardDisk $vhd -IDE -BUS 1 -LUN 0 | Out-Null
  } else {
      New-SCVirtualDiskDrive -VM $vm -VirtualHardDisk $vhd -SCSI -BUS 0 -LUN 62 | Out-Null
  }

  Remove-SCVirtualHardDisk -FloppyDisk $vhd -RunAsynchronously | out-null
  return VMToJson $vm "AddingVHD"
} catch {
  ErrorToJson 'Add VHD to VM' $_
}
