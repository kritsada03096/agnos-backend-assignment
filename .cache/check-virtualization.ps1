$ErrorActionPreference = 'Stop'
$report = [ordered]@{}
$report.Features = @(Get-WindowsOptionalFeature -Online -FeatureName VirtualMachinePlatform | Select-Object FeatureName,State)
$report.Features += @(Get-WindowsOptionalFeature -Online -FeatureName Microsoft-Windows-Subsystem-Linux | Select-Object FeatureName,State)
$report.Boot = @(& bcdedit.exe /enum)
$report.HypervisorPresent = (Get-CimInstance Win32_ComputerSystem).HypervisorPresent
$report.RebootPending = Test-Path 'HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending'
$report | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath 'C:\Users\krits\agnos-backend-assignment\.cache\virtualization-status.json' -Encoding UTF8
