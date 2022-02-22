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
  $vmjson.VirtualDisks = @($vm.VirtualHardDisks | select Size, MaximumSize)
}
if ($vm.BiosGuid -ne $null) { $vmjson.BiosGuid = "$($vm.BiosGuid)" }
if ($vm.Id -ne $null) { $vmjson.Id = "$($vm.Id)" }
if ($vm.VMId -ne $null) { $vmjson.VMId = "$($vm.VMId)" }
if ($vm.CreationTime -ne $null) { $vmjson.CreationTime = $vm.CreationTime.ToString('o') }
if ($vm.ModifiedTime -ne $null) { $vmjson.ModifiedTime = $vm.ModifiedTime.ToString('o') }
if ($message) { $vmjson.Message = $message }
$vmjson | convertto-json -Depth 2 -Compress
