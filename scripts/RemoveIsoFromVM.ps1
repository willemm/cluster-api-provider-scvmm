param($id, $isopath)
try {
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    throw "Virtual Machine $id not found"
  }
  $DVDDrive = Get-SCVirtualDVDDrive -VM $vm | Where-Object { $_.ISO.SharePath -eq $isopath } | select -first 1
  if (-not $DVDDrive) {
    throw "Isofile $isopath not connected to $id $($vm.name)"
  }
  $ISO = $DVDDrive.ISO
  Remove-SCISO -ISO $ISO | out-null

  return VMToJson $vm "Removing ISO"
} catch {
  ErrorToJson 'Remove ISO from VM' $_
}
