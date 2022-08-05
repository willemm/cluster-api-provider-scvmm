param($vmname, $id)
try {
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    throw "VM with id $id not found"
  }
  $vm = Set-SCVirtualMachine -Name $vmname -VM $vm -RunAsynchronously
  return VMToJson $vm
} catch {
  ErrorToJson 'Get VM' $_
}
