param($vm, $message = "")

$vmjson = @{}
if ($vm.Cloud -ne $null) { $vmjson.Cloud = $vm.Cloud.Name }
if ($vm.Name -ne $null) { $vmjson.Name = $vm.Name }
if ($vm.Status -ne $null) { $vmjson.Status = "$($vm.Status)" }
if ($vm.Memory -ne $null) { $vmjson.Memory = $vm.Memory }
if ($vm.CpuCount -ne $null) { $vmjson.CpuCount = $vm.CpuCount }
if ($vm.VirtualNetworkAdapters -ne $null) {
  $vmjson.VirtualNetwork = $vm.VirtualNetworkAdapters.VMNetwork.Name | select -first 1
  if ($vm.VirtualNetworkAdapters.IPv4Addresses) {
    $vmjson.IPv4Addresses = @($vm.VirtualNetworkAdapters.IPv4Addresses)
  }
  if ($vm.VirtualNetworkAdapters.Name) {
    $vmjson.Hostname = $vm.VirtualNetworkAdapters.Name | select -first 1
  }
}
if ($vm.VirtualHardDisks -ne $null) {
  $vmjson.VirtualDisks = $vm.VirtualDiskDrives | Foreach-Object {
    @{
      Size = $_.VirtualHardDisk.Size
      MaximumSize = $_.VirtualHardDisk.MaximumSize
      SharePath = $_.VirtualHardDisk.SharePath
      IOPSMaximum = $_.IOPSMaximum
    }
  }
  @($vm.VirtualHardDisks | select Size, MaximumSize, SharePath)
}
if ($vm.VirtualDVDDrives -ne $null) {
  $vmjson.ISOs = @($vm.VirtualDVDDrives.ISO | select Size, SharePath)
}
if ($vm.BiosGuid -ne $null) { $vmjson.BiosGuid = "$($vm.BiosGuid)" }
if ($vm.Id -ne $null) { $vmjson.Id = "$($vm.Id)" }
if ($vm.VMId -ne $null) { $vmjson.VMId = "$($vm.VMId)" }
if ($vm.AvailabilitySetNames -ne $null) { $vmjson.AvailabilitySetNames = $vm.AvailabilitySetNames }
if ($vm.CreationTime -ne $null) { $vmjson.CreationTime = $vm.CreationTime.ToString('o') }
if ($vm.ModifiedTime -ne $null) { $vmjson.ModifiedTime = $vm.ModifiedTime.ToString('o') }
if ($vm.Tag -ne $null) { $vmjson.Tag = "$($vm.Tag)" }
if ($vm.CustomProperty -ne $null) { $vmjson.CustomProperty = $vm.CustomProperty }
if ($message) { $vmjson.Message = $message }
$vmjson | convertto-json -Depth 2 -Compress
