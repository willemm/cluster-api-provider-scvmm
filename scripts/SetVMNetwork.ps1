param($id, $networkdevices)
try {
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    throw "Virtual Machine $id not found"
  }
  if ($vm.MostRecentTask -and $vm.MostRecentTask.Status -ne 'Completed') {
    return VMToJson $vm "Machine is busy"
  }
  $networklist = $networkdevices | ConvertFrom-Json
  foreach ($vna in $vm.VirtualNetworkAdapters) {
    $nd = $networklist[$vna.SlotId]
    if ($nd) {
      if ($nd.staticIpAddressPools) {
        foreach ($apname in $networkdevice.staticIPAddressPools) {
          $ap = Get-SCStaticIPAddressPool $apname
          if (-not $ap) {
            throw "StaticIPAddressPool $apname not found"
          }
          Grant-SCIPAddress -GrantToObjectType "VirtualNetworkAdapter" -GrantToObjectID $vna.ID -StaticIPAddressPool $ap | Out-Null
          if ($ap.AddressFamily -eq 'InterNetwork') {
            Set-SCVirtualNetworkAdapter -VirtualNetworkAdapter $vna -IPv4AddressType static | Out-Null
          }
          if ($ap.AddressFamily -eq 'InterNetworkV6') {
            Set-SCVirtualNetworkAdapter -VirtualNetworkAdapter $vna -IPv6AddressType static | Out-Null
          }
        }
      }
    }
  }
  return VMToJson $vm "Nothing"
} catch {
  ErrorToJson 'Expand VM Disks' $_
}
