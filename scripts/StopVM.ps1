param($id)
try {
  $vm = Stop-SCVirtualMachine -ID $id -RunAsynchronously
  return VMToJson $vm "Stopping"
} catch {
  ErrorToJson 'Stop VM' $_
}
