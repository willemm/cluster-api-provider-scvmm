param($vmname)
try {
  $vm = Get-SCVirtualMachine -Name $vmname
  if (-not $vm) {
    return @{ Message = "VM $($vmname) not found" } | convertto-json
  }
  return VMToJson $vm
} catch {
  ErrorToJson 'Get VM' $_
}
