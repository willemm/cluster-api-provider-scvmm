param($vmname, $availabilityset)
try {
  $vm = Get-SCVirtualMachine -Name $vmname
  if (-not $vm) {
    return @{ Message = "VM $($vmname) not found" } | convertto-json
  }
  if ($availabilityset) {
    $vm | Set-SCVirtualMachine -AvailabilitySetNames $availabilityset
  }
  $vm = Start-SCVirtualMachine -VM $vmname -RunAsynchronously
  return VMToJson $vm "Starting"
} catch {
  ErrorToJson 'Start VM' $_
}
