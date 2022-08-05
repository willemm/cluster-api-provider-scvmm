param($vmname)
try {
  $vm = Get-SCVirtualMachine -Name $vmname
  if (-not $vm) {
    return @{ Message = "VM $($vmname) not found" } | convertto-json
  }
  $vm = Start-SCVirtualMachine -VM $vmname -RunAsynchronously
  return VMToJson $vm "Starting"
} catch {
  ErrorToJson 'Start VM' $_
}
