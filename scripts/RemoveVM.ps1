param($vmname)
try {
  $vm = Get-SCVirtualMachine $vmname
  if (-not $vm) {
    return (@{ Message = "Removed" } | convertto-json)
  }
  if ($vm.Status -eq 'PowerOff') {
    foreach ($vdd in $vm.VirtualDVDDrives) {
      if ($vdd.ISO -and $vdd.ISO.SharePath -match "\\$($vm.Name)-cloud-init\.iso`$") {
        $ISO = $vdd.ISO
        Set-SCVirtualDVDDrive -VirtualDVDDrive $vdd -NoMedia | out-null
        Remove-SCISO -ISO $ISO | out-null
      }
    }
    $vm = Remove-SCVirtualMachine $vm -RunAsynchronously
    VMToJson $vm "Removing"
  } else {
    $vm = Stop-SCVirtualmachine $vm -RunAsynchronously
    VMToJson $vm "Stopping"
  }
} catch {
  ErrorToJson 'Remove VM' $_
}
