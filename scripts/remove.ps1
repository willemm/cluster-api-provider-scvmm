function RemoveVM($vmname) {
  try {
    $vm = Get-SCVirtualMachine $vmname
    if (-not $vm) {
      return (@{ Message = "Removed" } | convertto-json)
    }
    if ($vm.Status -eq 'PowerOff') {
      $vm = Remove-SCVirtualMachine $vmname
      VMToJson($vm, "Removing")
    } else {
      $vm = Stop-SCVirtualmachine $vm
      VMToJson($vm, "Stopping")
    }
  } catch {
    ErrorToJson('Remove VM', $_)
  }
}
