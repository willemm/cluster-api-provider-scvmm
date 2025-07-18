param($vmhost, $path, $filename)
try {
  $vm = New-SCVirtualMachine -VMHost $vmhost -Path $path -Name $filename
  try {
	  $vd = New-SCVirtualDiskDrive -UseLocalVirtualHardDisk -Path $path -FileName $filename -IDE -BUS 0 -LUN 0 VM $vm
  } catch {
	  if ($_.Exception.Message -match 'could not locate the specified file') {
		  $vm = Remove-SCVirtualMachine -VM $vm
		  return VMToJson $vm "Not Found"
	  }
  }
  $vm = Remove-SCVirtualMachine -VM $vm
  return VMToJson $vm "Removing PersistentDisk"
} catch {
  if ($vm) {
    try {
      Remove-SCVirtualMachine -VM $vm | Out-Null
    } catch {
    }
  }
  ErrorToJson 'Remove PersistentDisk' $_
}
