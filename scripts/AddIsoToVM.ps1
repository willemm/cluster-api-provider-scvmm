param($vmname, $isopath)
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
