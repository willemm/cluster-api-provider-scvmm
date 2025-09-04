param($id, $cipath, $devicetype)
try {
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    throw "Virtual Machine with ID $id not found"
  }
  if ($vm.MostRecentTask -and $vm.MostRecentTask.Status -ne 'Completed') {
    return VMToJson $vm "Machine is busy"
  }
  $shr = Get-SCLibraryShare | ?{ $cipath.StartsWith($_.Path) } | select -first 1
  if (-not $shr) {
    throw "Library share containing $cipath not found"
  }
  $pdir = split-path ($cipath.Remove(0,$shr.Path.length+1))
  Read-SCLibraryShare -LibraryShare $shr -Path $pdir | out-null
  $vfd = Get-SCVirtualFloppyDisk | Where-Object { $_.SharePath -eq $cipath } | select -first 1
  if (-not $vfd) {
    throw "Firtualfloppydisk $cipath not found"
  }
  $FloppyDrive = Get-SCVirtualFloppyDrive -VM $vm | select -first 1
  if (-not $FloppyDrive) {
      $FloppyDrive = New-SCVirtualFloppyDrive -VM $vm
  }
  if ($FloppyDrive.VirtualFloppyDisk) {
    Set-SCVirtualFloppyDrive -VirtualFloppyDrive $FloppyDrive -NoMedia | out-null
  }
  Set-SCVirtualFloppyDrive -VirtualFloppyDrive $FloppyDrive -VirtualFloppyDisk $vfd | out-null

  Remove-SCVirtualFloppyDisk -VirtualFloppyDisk $vfd -RunAsynchronously | out-null
  return VMToJson $vm "AddingVFD"
} catch {
  ErrorToJson 'Add VFD to VM' $_
}
