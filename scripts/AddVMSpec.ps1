param($spec, $metadata)
try {
  return '{}'
} catch {
  ErrorToJson 'Add VM Spec' $_
}
