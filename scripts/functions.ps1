function VMToJson($vm, $message = "") {
  @{
    Cloud = $vm.Cloud.Name
    Name = $vm.Name
    Status = "$($vm.Status)"
    Memory = $vm.Memory
    CpuCount = $vm.CpuCount
    VirtualNetwork = $vm.VirtualNetworkAdapters.VMNetwork.Name
    Guid = $vm.BiosGuid
    CreationTime = $vm.CreationTime.ToString('o')
    ModifiedTime = $vm.ModifiedTime.ToString('o')
    Message = $message
  } | convertto-json
}

function ErrorToJson($what,$err) {
  @{
    Message = "$($what) Failed: $($err.Exception.Message)"
    Error = "$($err.Exception)"
  } | convertto-json
}
