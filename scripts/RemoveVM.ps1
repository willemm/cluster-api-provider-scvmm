param($vmname)
try {
  $vm = Get-SCVirtualMachine $vmname
  if (-not $vm) {
    return (@{ Message = "Removed" } | convertto-json)
  }
  if ($vm.Status -eq 'PowerOff') {
    $vm = Remove-SCVirtualMachine $vm -RunAsynchronously
    VMToJson $vm "Removing"
  } else {
    $vm = Stop-SCVirtualmachine $vm -RunAsynchronously
    VMToJson $vm "Stopping"
  }
} catch {
  ErrorToJson 'Remove VM' $_
}
