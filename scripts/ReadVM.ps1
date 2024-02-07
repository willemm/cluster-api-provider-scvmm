param($id)
try {
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    return @{ Message = "VM $($id) not found" } | convertto-json
  }
  $vm = Read-SCVirtualMachine -vm $vm -RunAsynchronously
  return VMToJson $vm
} catch {
  ErrorToJson 'Get VM' $_
}
