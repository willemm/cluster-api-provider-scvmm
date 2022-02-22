param($name, $oupath, $domaincontroller)
try {
  $dcparam = @{}
  if ($domaincontroller) {
    $dcparam.Server = $domaincontroller
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
