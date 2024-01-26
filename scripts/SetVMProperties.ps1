param($id, $tag, $customproperty)
try {
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    throw "Virtual Machine with ID $id not found"
  }

  if ($tag) {
    Set-SCVirtualMachine -VM $vm -Tag $tag -RunAsynchronously -ErrorAction Stop
  }
  if ($customproperty) {
    $properties = $customproperty | ConvertFrom-Json
    foreach ($prop in $properties.keys) {
      $cp = Get-SCCustomProperty -Name $prop
      if ($cp) {
        Set-SCCustomPropertyValue -InputObject $vm -CustomProperty $cp -Value $properties[$prop] -RunAsynchronously -ErrorAction Stop
      }
    }
  }

  return VMToJson $vm "Setting Properties"
} catch {
  ErrorToJson 'Create VM' $_
}

