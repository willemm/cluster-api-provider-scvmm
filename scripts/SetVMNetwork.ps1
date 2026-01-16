param($id, $networkdevices)
try {
  $vm = Get-SCVirtualMachine -ID $id
  if (-not $vm) {
    throw "Virtual Machine $id not found"
  }
  if ($vm.MostRecentTask -and $vm.MostRecentTask.Status -notin 'Completed','SucceedWithInfo') {
    return VMToJson $vm "Machine is busy"
  }
  $networklist = $networkdevices | ConvertFrom-Json
  foreach ($vna in $vm.VirtualNetworkAdapters) {
    $nd = $networklist[$vna.SlotId]
    if ($nd) {
      if ($nd.staticIpAddressPools) {
        $apv4 = @()
        $apv6 = @()
        foreach ($apname in $nd.staticIPAddressPools) {
          $ap = Get-SCStaticIPAddressPool $apname
          if (-not $ap) {
            throw "StaticIPAddressPool $apname not found"
          }
          #Grant-SCIPAddress -GrantToObjectType "VirtualNetworkAdapter" -GrantToObjectID $vna.ID -StaticIPAddressPool $ap | Out-Null
          if ($ap.AddressFamily -eq 'InterNetwork') {
            $apv4 += $ap
          }
          if ($ap.AddressFamily -eq 'InterNetworkV6') {
            $apv6 += $ap
          }
        }
        if ($apv4 -and $vna.IPv4AddressType -ne 'Static') {
          Set-SCVirtualNetworkAdapter -VirtualNetworkAdapter $vna -IPv4AddressType static -IPv4AddressPools $apv4 | Out-Null
          return VMToJson $vm "Set static IPv4"
        }
        if ($apv6 -and $vna.IPv6AddressType -ne 'Static') {
          Set-SCVirtualNetworkAdapter -VirtualNetworkAdapter $vna -IPv6AddressType static -IPv6AddressPools $apv6 | Out-Null
          return VMToJson $vm "Set static IPv6"
        }
      }
    }
  }
  return VMToJson $vm "Nothing"
} catch {
  ErrorToJson 'Expand VM Disks' $_
}
