param($name, $oupath, $domaincontroller)
try {
  $dcparam = @{}
  if ($domaincontroller) {
    $dcparam.Server = $domaincontroller
  }
  if (${env:ACTIVEDIRECTORY_USERNAME} -and ${env:ACTIVEDIRECTORY_PASSWORD}) {
    $dcparam.Credential = new-object PSCredential(${env:ACTIVEDIRECTORY_USERNAME}, (ConvertTo-Securestring -force -AsPlainText -String ${env:ACTIVEDIRECTORY_PASSWORD}))
  }
  $comp = Get-ADComputer @dcparam -LDAPFilter "(samaccountname=$($name)`$)"
  if (-not $comp) {
    return @{ Message = "ADComputer $($name) not found" } | convertto-json
  }
  $comp | Remove-ADComputer @dcparam -Confirm:$false
  return @{ Message = "ADComputer $($name) removed" } | convertto-json
} catch {
  ErrorToJson 'Remove AD Computer' $_
}
