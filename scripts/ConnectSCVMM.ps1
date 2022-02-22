param($host, $username, $password)
$credparam = @{ Credential = new-object PSCredential($username, (ConvertTo-Securestring -force -AsPlainText -String $password)) }
Get-SCVmmServer $host @credparam | out-null
