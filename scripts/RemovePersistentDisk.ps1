param($vmhost, $path, $filename)
try {
  $JobGroupID = [GUID]::NewGuid().ToString()
  New-SCVirtualDiskDrive -UseLocalVirtualHardDisk -Path $path -FileName $filename -IDE -BUS 0 -LUN 0 -JobGroup $JobGroupID
  $vm = New-SCVirtualMachine -VMHost $vmhost -Path $path -Name $filename -JobGroup $JobGroupID -RunAsynchronously
  $vm = Remove-SCVirtualMachine -VM $vm -RunAsynchronously
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
