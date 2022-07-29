param($vmname, $availabilityset)
try {
  $vm = Get-SCVirtualMachine -Name $vmname
  if (-not $vm) {
    return @{ Message = "VM $($vmname) not found" } | convertto-json
  }
  $vm | Set-SCVirtualMachine -AvailabilitySetNames $availabilityset
  return VMToJson $vm "AddAvailabilityset"
} catch {
  ErrorToJson 'Add Availabilityset' $_
}
