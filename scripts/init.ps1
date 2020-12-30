$cred = new-object PSCredential('${SCVMM_USERNAME}', (ConvertTo-Securestring -force -AsPlainText -String '${SCVMM_PASSWORD}'))
Get-SCVmmServer ${SCVMM_HOST} -Credential $cred | out-null
