$credparam = @{ Credential = new-object PSCredential('${SCVMM_USERNAME}', (ConvertTo-Securestring -force -AsPlainText -String '${SCVMM_PASSWORD}')) }
Get-SCVmmServer ${SCVMM_HOST} @credparam | out-null
