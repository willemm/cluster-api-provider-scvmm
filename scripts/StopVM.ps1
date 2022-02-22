param($vmname)
try {
  $vm = Stop-SCVirtualMachine -VM $vmname -RunAsynchronously
  return VMToJson $vm "Stopping"
} catch {
  ErrorToJson 'Stop VM' $_
}
