param(
    [string]$BaseUrl = 'http://localhost:8080',
    [string]$ProvisioningKey = 'local-demo-provisioning-key-change-before-deploy-123456'
)
$ErrorActionPreference = 'Stop'
$suffix = [Guid]::NewGuid().ToString('N').Substring(0, 12)
$tokens = @{}
foreach ($hospital in @('hospital-a', 'hospital-b')) {
    $body = @{ username = "smoke-$suffix"; password = 'smoke-test-password'; hospital = $hospital } | ConvertTo-Json
    $created = Invoke-RestMethod -Method Post -Uri "$BaseUrl/staff/create" -ContentType 'application/json' -Headers @{ 'X-Provisioning-Key' = $ProvisioningKey } -Body $body
    if (-not $created.success) { throw 'Staff creation failed' }
    $login = Invoke-RestMethod -Method Post -Uri "$BaseUrl/staff/login" -ContentType 'application/json' -Body $body
    $tokens[$hospital] = $login.data.access_token
}
$results = @{}
foreach ($hospital in @('hospital-a', 'hospital-b')) {
    $headers = @{ Authorization = "Bearer $($tokens[$hospital])" }
    $result = Invoke-RestMethod -Uri "$BaseUrl/patient/search?national_id=1111111111111" -Headers $headers
    $expectedHN = if ($hospital -eq 'hospital-a') { 'HN-a-001' } else { 'HN-b-001' }
    if ($result.data.Count -ne 1 -or $result.data[0].patient_hn -ne $expectedHN) { throw "Incorrect patient for $hospital" }
    $results[$hospital] = $result.data[0].hospital_id
    $nameResult = Invoke-RestMethod -Uri "$BaseUrl/patient/search?first_name=Demo" -Headers $headers
    if ($nameResult.data.Count -lt 1) { throw 'Name search did not find imported patient' }
    $mismatch = Invoke-RestMethod -Uri "$BaseUrl/patient/search?national_id=1111111111111&first_name=NoMatch" -Headers $headers
    if ($mismatch.data.Count -ne 0) { throw 'Search ignored extra filter' }
}
if ($results['hospital-a'] -eq $results['hospital-b']) { throw 'Hospital isolation failed' }
Write-Output 'Smoke test passed: create, login, HIS import, name search, filters, hospital isolation.'
