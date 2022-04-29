param($vmname)
try {
  $vm = Get-SCVirtualMachine $vmname
  if (-not $vm) {
    return (@{ Message = "Removed" } | convertto-json)
  }
  if ($vm.Status -eq 'PowerOff') {
    $JobGroupID = [GUID]::NewGuid().ToString()
    $ISOs = $vm.VirtualDVDDrives.ISO | ?{ $_.SharePath -match '-cloud-init\.iso$' }
    $vm = Remove-SCVirtualMachine $vm -RunAsynchronously -JobGroup $JobGroupID
    foreach ($iso in $ISOs) {
      Remove-SCISO -ISO $iso -RunAsynchronously -JobGroup $JobGroupID
    }
    VMToJson $vm "Removing"
  } else {
    $vm = Stop-SCVirtualmachine $vm -RunAsynchronously
    VMToJson $vm "Stopping"
  }
} catch {
  ErrorToJson 'Remove VM' $_
}
