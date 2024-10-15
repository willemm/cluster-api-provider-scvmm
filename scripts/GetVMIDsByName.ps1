param([string]$vmName)

# Get all VMs with the specified name; SCVMM will return a list if there are more than one
$vms = Get-SCVirtualMachine -Name $vmName
$vmIds = @()

# Check if any VMs were returned and collect their IDs
if ($vms) {
    foreach ($vm in $vms) {
        $vmIds += $vm.ID
    }
}

# VMIDs ([]string) member was added to VMResult so this can use the same invoke/marshall logic as other scripts
@{ VMIDs = $vmIds } | ConvertTo-Json