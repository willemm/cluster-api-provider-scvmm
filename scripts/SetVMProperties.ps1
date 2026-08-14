param($id, $tag, $customproperty, $vmoptions)
try {
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    throw "Virtual Machine with ID $id not found"
  }
  if ($vm.MostRecentTask -and $vm.MostRecentTask.Status -notin 'Completed','SucceedWithInfo') {
    return VMToJson $vm "Machine is busy"
  }

  $vmargs = @{}
  $optionsobject = $vmoptions | ConvertFrom-Json
  foreach ($optkey in @('HAVMPriority','EnableBackup')) {
    if ($optionsobject.$optkey -ne $null) { $vmargs[$optkey] = $optionsobject.$optkey }
  }
  if ($tag) {
    $vmargs['Tag'] = $tag
  }
  if ($vmargs) {
    Set-SCVirtualMachine -VM $vm @vmargs -RunAsynchronously -ErrorAction Stop | Out-Null
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

