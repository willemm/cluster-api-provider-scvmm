param($name, $oupath, $domaincontroller, $description, $memberof, $username, $password)
try {
  $dcparam = @{}
  if ($domaincontroller) {
    $dcparam.Server = $domaincontroller
  }
  if ($username -and $password) {
    $dcparam['Credential'] = new-object PSCredential($username, (ConvertTo-Securestring -force -AsPlainText -String $password))
  }
  $ident = "CN=$($name),$($oupath)"
  try {
    $comp = Get-ADComputer @dcparam -Identity $ident
    if (-not $comp) {
      throw "ADComputer $ident not found"
    }
  } catch {
    New-ADComputer @dcparam -Name $name -Path $oupath -AccountPassword $null -samaccountname "$($name)`$" -enabled $true -Description $description
  }
  foreach ($grp in $memberof) {
    Add-ADGroupMember @dcparam -Identity $grp -Members $ident
  }
  return @{ Message = "ADComputer $($ident) created" } | convertto-json
} catch {
  ErrorToJson 'Create AD Computer' $_
}
