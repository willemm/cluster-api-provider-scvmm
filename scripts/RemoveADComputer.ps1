param($name, $oupath, $domaincontroller)
try {
  $dcparam = @{}
  if ($domaincontroller) {
    $dcparam.Server = $domaincontroller
  }
  if (${env:ACTIVEDIRECTORY_USERNAME} -and ${env:ACTIVEDIRECTORY_PASSWORD}) {
    $dcparam.Credential = new-object PSCredential(${env:ACTIVEDIRECTORY_USERNAME}, (ConvertTo-Securestring -force -AsPlainText -String ${env:ACTIVEDIRECTORY_PASSWORD}))
  }
  $ident = "CN=$($name),$($oupath)"
  try {
    $comp = Get-ADComputer @dcparam -Identity $ident
  } catch {
    return @{ Message = "ADComputer $($ident) not found" } | convertto-json
  }
  Remove-ADComputer @dcparam -Confirm:$false -Identity $ident
  return @{ Message = "ADComputer $($ident) removed" } | convertto-json
} catch {
  ErrorToJson 'Remove AD Computer' $_
}
