param($id)
try {
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    return @{ Message = "VM $($id) not found" } | convertto-json
  }
  Read-SCVirtualMachine -vm $vm | out-null
  return VMToJson $vm
} catch {
  ErrorToJson 'Get VM' $_
}
