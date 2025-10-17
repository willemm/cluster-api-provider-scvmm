param($id)
try {
  if (-not $id) {
    return (@{ Message = "Removed" } | convertto-json)
  }
  $vm = Get-SCVirtualMachine -ID $id -ErrorAction SilentlyContinue
  if (-not $vm) {
    return (@{ Message = "Removed" } | convertto-json)
  }
  if ($vm.Status -eq 'Running') {
    $vm = Stop-SCVirtualmachine $vm -Force -RunAsynchronously
    VMToJson $vm "Stopping"
  } else if ($vm.Status -eq 'Stopping') {
    VMToJson $vm "Stopping"
  } else {
    $vm = Remove-SCVirtualMachine $vm -RunAsynchronously
    VMToJson $vm "Removing"
  }
} catch {
  ErrorToJson 'Remove VM' $_
}
