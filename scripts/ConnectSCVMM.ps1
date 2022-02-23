param($computername, $username, $password)
$credparam = @{ Credential = new-object PSCredential($username, (ConvertTo-Securestring -force -AsPlainText -String $password)) }
Get-SCVmmServer -ComputerName $computername @credparam | out-null
