param($id)
try {
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    return @{ Message = "VM $($id) not found" } | convertto-json
  }
  if ($vm.MostRecentTask -and $vm.MostRecentTask.Status -notin 'Completed','SucceedWithInfo') {
    return VMToJson $vm "Machine is busy"
  }
  $vm = Start-SCVirtualMachine -VM $vm -RunAsynchronously
  return VMToJson $vm "Starting"
} catch {
  ErrorToJson 'Start VM' $_
}
