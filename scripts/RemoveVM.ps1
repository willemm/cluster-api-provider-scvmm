param($id)
try {
  if (-not $id) {
    return (@{ Message = "Removed" } | convertto-json)
  }
  $vm = Get-SCVirtualMachine -ID $id -ErrorAction SilentlyContinue
  if (-not $vm) {
    return (@{ Message = "Removed" } | convertto-json)
  }
  if ($vm.Status -eq 'PowerOff') {
    $vm = Remove-SCVirtualMachine $vm -RunAsynchronously
    VMToJson $vm "Removing"
  } else {
    $vm = Stop-SCVirtualmachine $vm -Force -RunAsynchronously
    VMToJson $vm "Stopping"
  }
} catch {
  ErrorToJson 'Remove VM' $_
}
