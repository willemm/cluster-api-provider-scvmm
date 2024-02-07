param($id, $tag, $customproperty)
try {
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    throw "Virtual Machine with ID $id not found"
  }

  if ($tag) {
    Set-SCVirtualMachine -VM $vm -Tag $tag -RunAsynchronously -ErrorAction Stop | Out-Null
  }
  if ($customproperty) {
    $properties = $customproperty | ConvertFrom-Json
    if ($properties) {
      foreach ($prop in ($properties | Get-Member -Type NoteProperty).Name) {
	$cp = Get-SCCustomProperty -Name $prop
	if (-not $cp) {
	  throw "Custom Property $prop not found"
	}
	if ($cp) {
	  Set-SCCustomPropertyValue -InputObject $vm -CustomProperty $cp -Value $properties.$prop -RunAsynchronously -ErrorAction Stop | Out-Null
	}
      }
    }
  }
  $vm = Read-SCVirtualMachine -VM $vm -RunAsynchronously
  return VMToJson $vm "Setting Properties"
} catch {
  ErrorToJson 'Set VM Properties' $_
}

