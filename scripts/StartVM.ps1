param($vmname)
try {
  $vm = Start-SCVirtualMachine -VM $vmname -RunAsynchronously
  return VMToJson $vm "Starting"
} catch {
  ErrorToJson 'Start VM' $_
}
