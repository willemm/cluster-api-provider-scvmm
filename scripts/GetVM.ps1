param($id)
try {
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    return @{ Message = "VM with id $($id) not found" } | convertto-json
  }
  return VMToJson $vm
} catch {
  ErrorToJson 'Get VM' $_
}
