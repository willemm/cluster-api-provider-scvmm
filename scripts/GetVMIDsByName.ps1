param([string]$vmName)
try {
    # Get all VMs with the specified name; SCVMM will return a list if there is more than one
    $vms = Get-SCVirtualMachine -Name $vmName
    $vmIds = @()

    # Check if any VMs were returned and collect their IDs
    if ($vms) {
        foreach ($vm in $vms) {
            $vmIds += $vm.ID
        }
    }

    @{ VMIDs = $vmIds } | ConvertTo-Json -Compress
} catch {
    ErrorToJson 'Get VMIDs by VMName' $_
}
