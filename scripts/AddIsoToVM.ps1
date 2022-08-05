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
  if ($DVDDrive.ISO) {
    Set-SCVirtualDVDDrive -VirtualDVDDrive $DVDDrive -NoMedia | out-null
  }
  Set-SCVirtualDVDDrive -VirtualDVDDrive $DVDDrive -ISO $ISO | out-null

  Remove-SCISO -ISO $ISO -RunAsynchronously | out-null
  return VMToJson $vm "AddingISO"
} catch {
  ErrorToJson 'Add ISO to VM' $_
}
