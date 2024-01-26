param($vmname, $id)
try {
  if (-not $id) {
    return @{ Message = "VM $($vmname) not found, no ID given" } | convertto-json
  }
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    return @{ Message = "VM $($vmname) with id $($id) not found" } | convertto-json
  }
  $vmlist = Get-SCVirtualMachine -Name $vmname
  $message = ""
  if ($vmlist.Count -gt 1) {
    # If one of the vms has an ISO coupled, it already won the race
    $realvm = $vmlist | Where-Object { $_.VirtualDVDDrives[0].ISO -ne $null }
    if ($realvm.Count -gt 1) {
      throw "Fatal: More than one vm with name $vmname that have an ISO coupled"
    }
    if ($realvm) {
      $firstvm = $realvm | Select-Object -First 1
      $message = "ISO present so this is the real vm"
    } else {
      # If more VMs exist, check if we have the lowest ID
      $firstvm = $vmlist | Sort-Object ID | Select-Object -First 1
      # 'may be taken' means another vm has to give up the name.
      # But we have to wait for that because that other vm might have
      # been alone when it checked and is now busy coupling an ISO.
      $message = "VMName may be taken"
    }
    if ($firstvm.ID -ne $id) {
      # 'is taken' means we have to give up the name
      $message = "VMName is taken"
    }
  }
  return VMToJson $vm $message
} catch {
  ErrorToJson 'Get VM' $_
}
