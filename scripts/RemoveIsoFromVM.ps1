param($vmname, $isopath)
try {
  $vm = Get-SCVirtualMachine -Name $vmname
  if (-not $vm) {
    throw "Virtual Machine $vmname not found"
  }
  $DVDDrive = Get-SCVirtualDVDDrive -VM $vm | Where-Object { $_.ISO.SharePath -eq $isopath } | select -first 1
  if (-not $DVDDrive) {
    throw "Isofile $isopath not connected to $vmname"
  }
  $ISO = $DVDDrive.ISO
  Set-SCVirtualDVDDrive -VirtualDVDDrive $DVDDrive -NoMedia | out-null
  Remove-SCISO -ISO $ISO | out-null

  return VMToJson $vm "Removing ISO"
} catch {
  ErrorToJson 'Remove ISO from VM' $_
}
