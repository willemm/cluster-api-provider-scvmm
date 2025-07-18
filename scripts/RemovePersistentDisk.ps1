param($vmhost, $path, $filename)
try {
  $vm = New-SCVirtualMachine -VMHost $vmhost -Path $path -Name "rm-$filename"
  $err = $null
  $vd = New-SCVirtualDiskDrive -UseLocalVirtualHardDisk -Path $path -FileName $filename -IDE -BUS 0 -LUN 0 -VM $vm -ErrorAction SilentlyContinue -ErrorVariable err
  if ($err) {
    $vm = Remove-SCVirtualMachine -VM $vm
    if ($err.Exception.Message -match 'could not locate the specified file') {
      return VMToJson $vm "Not Found"
    }
    return ErrorToJson 'Remove PersistentDisk' $err
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
