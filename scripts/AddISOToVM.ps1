param($id, $cipath, $devicetype)
try {
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    throw "Virtual Machine with ID $id not found"
  }
  if ($vm.MostRecentTask -and $vm.MostRecentTask.Status -notin 'Completed','SucceedWithInfo') {
    return VMToJson $vm "Machine is busy"
  }
  $shr = Get-SCLibraryShare | ?{ $cipath.StartsWith($_.Path) } | select -first 1
  if (-not $shr) {
    throw "Library share containing $cipath not found"
  }
  $pdir = split-path ($cipath.Remove(0,$shr.Path.length+1))
  Read-SCLibraryShare -LibraryShare $shr -Path $pdir | out-null
  $ISO = Get-SCISO | Where-Object { $_.SharePath -eq $cipath } | select -first 1
  if (-not $ISO) {
    throw "Isofile $cipath not found"
  }
  $DVDDrive = Get-SCVirtualDVDDrive -VM $vm | select -first 1
  if ($DVDDrive.ISO) {
    Set-SCVirtualDVDDrive -VirtualDVDDrive $DVDDrive -NoMedia | out-null
  }
  Set-SCVirtualDVDDrive -VirtualDVDDrive $DVDDrive -ISO $ISO | out-null

  Remove-SCISO -ISO $ISO -RunAsynchronously | out-null
  return VMToJson $vm "AddingISO"
} catch {
  ErrorToJson 'Add ISO to VM' $_
}
