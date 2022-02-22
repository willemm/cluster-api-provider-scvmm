param($spec, $metadata)
try {
  $specobj = $spec | convertfrom-json
  $metadataobj = $metadata | convertfrom-json
  $newspec = @{}
  if (-not $specobj.VMName) {
    $namerange = $metadataobj.annotations.'scvmmmachine.cluster.x-k8s.io/vmnames'
    if ($namerange) {
      $rstart, $rend = $namerange -split ':'
      $vmname = $rstart
      while ($vmname -le $rend) {
        if (-not (Get-SCVirtualMachine -Name $vmname)) { break }
        $nextname = ""
        $brk = $false
        for ($vi = $vmname.length-1; $vi -ge 0; $vi = $vi-1) {
          $chr = $vmname[$vi]
          if (-not $brk) {
            if ($chr -match '[0-8A-Ya-y]') {
              $chr = [char]([int]$chr+1)
              $brk = $true
            } elseif ($chr -eq '9') {
              $chr = '0'
            } else {
              $chr=[char]([int]$chr-25)
            }
          }
          $nextname = "$chr$nextname"
        }
        $vmname = $nextname
      }
      if ($vmname -le $rend) {
        $newspec.VMName = $vmname
      } else {
        throw "no vmname available in range $namerange"
      }
    } else {
      $newspec.VMName = $metadataobj.name
    }
  }
  return $newspec | convertto-json -depth 3 -compress
} catch {
  ErrorToJson 'Generate VM Name' $_
}
