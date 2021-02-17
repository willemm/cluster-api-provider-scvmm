function AddVMSpec($spec, $metadata) {
  try {
    return '{"spec":'+$spec+'}'
  } catch {
    ErrorToJson 'Add VM Spec' $_
  }
}
