param($computername)
$credparam = @{}
if ($env:SCVMM_USERNAME -and $env:SCVMM_PASSWORD) {
  $credparam.Credential = new-object PSCredential($env:SCVMM_USERNAME, (ConvertTo-Securestring -force -AsPlainText -String $env:SCVMM_PASSWORD))
}
Get-SCVmmServer -ComputerName $computername @credparam | out-null
