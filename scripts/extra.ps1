function AddVMSpec($spec, $metadata) {
  try {
    return $spec
  } catch {
    ErrorToJson 'Add VM Spec' $_
  }
}
