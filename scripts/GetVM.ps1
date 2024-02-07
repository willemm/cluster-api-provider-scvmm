param($id)
try {
  $vm = Get-SCVirtualMachine -ID $id -ErrorAction SilentlyContinue
  if (-not $vm) {
    return @{ Message = "VM with id $($id) not found" } | convertto-json
  }
  return VMToJson $vm
} catch {
  ErrorToJson 'Get VM' $_
}
