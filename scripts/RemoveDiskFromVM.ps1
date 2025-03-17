param($id, $lun)
try {
  if (-not $id) {
    return (@{ Message = "No disk to remove" } | convertto-json)
  }
  $vm = Get-SCVirtualMachine -ID $id -ErrorAction SilentlyContinue
  if (-not $vm) {
    return (@{ Message = "No disk to remove" } | convertto-json)
  }
  if ($vm.Status -eq 'PowerOff') {
    foreach ($vhdisk in $vm.VirtualDiskDrives) {
      if ($vhdisk.LUN -eq $lun) {
	$vdd = Remove-SCVirtualDiskDrive -SkipDeleteVHD -VirtualDiskDrive $vhdisk -RunAsynchronously
	VMToJson $vm "Removing Disk $lun"
      }
    }
    VMToJson $vm "No disk to remove"
  } else {
    $vm = Stop-SCVirtualmachine $vm -Force -RunAsynchronously
    VMToJson $vm "Stopping"
  }
} catch {
  ErrorToJson 'Remove Disk' $_
}
