param($vmname, $id)
try {
  if ((-not $vmname) -and $id) {
    $vm = Get-SCVirtualMachine -ID $id
    return VMToJson $vm
  }
  $vm = Get-SCVirtualMachine -Name $vmname
  if (-not $vm) {
    return @{ Message = "VM $($vmname) not found" } | convertto-json
  }
  $message = ""
  if ($vm.Count -gt 1) {
    if (-not $id) {
      throw "Fatal: More than one vm with name $vmname and no ID provided"
    }
    $realvm = $vm | Where-Object { $_.VirtualDVDDrives[0].ISO -ne $null }
    if ($realvm.Count -gt 1) {
      throw "Fatal: More than one vm with name $vmname that have an ISO coupled"
    }
    if ($realvm) {
      $vm = $realvm | Select-Object -First 1
      $message = "ISO present so this is the real vm"
    } else {
      $vm = $vm | Sort-Object ID | Select-Object -First 1
      $message = "VMName may be taken"
    }
    if ($vm.ID -ne $id) {
      $message = "VMName is taken"
    }
  }
  return VMToJson $vm $message
} catch {
  ErrorToJson 'Get VM' $_
}
