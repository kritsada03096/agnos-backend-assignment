$ErrorActionPreference = 'Stop'
Restart-Service -Name 'agnos-postgresql16'
(Get-Service -Name 'agnos-postgresql16').WaitForStatus('Running', [TimeSpan]::FromSeconds(30))
