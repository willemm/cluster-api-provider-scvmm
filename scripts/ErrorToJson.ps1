param($what,$err)
@{
  Message = "$($what) Failed: $($err.Exception.Message)"
  Error = "$($err) $($err.ScriptStackTrace)"
} | convertto-json -Depth 2 -Compress
