param()
try {
  $ls = Get-SCLibraryShare | Where-Object { -not $_.IsViewOnly -and -not $_.MarkedForDeletion } | Select -First 1
  if (-not $ls) {
    throw "no library share found"
  }
  return @{ Result = "$($ls.Path)" } | convertto-json
} catch {
  ErrorToJson 'Get LibraryShare' $_
}
