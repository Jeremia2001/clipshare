$env:ENV = "development"
$env:CLIPSHARE_DEV = "1"

# Start Vite dev server in a new window so HMR works
$viteProcess = Start-Process -PassThru -FilePath "cmd.exe" `
    -ArgumentList "/c", "cd `"$PSScriptRoot\frontend`" && npm run dev"

# Poll until Vite is ready
Write-Host "Waiting for Vite on http://localhost:5173..."
$ready = $false
for ($i = 0; $i -lt 30; $i++) {
    try {
        $null = Invoke-WebRequest -Uri "http://localhost:5173" -UseBasicParsing -TimeoutSec 1 -ErrorAction Stop
        $ready = $true
        break
    } catch {}
    Start-Sleep -Seconds 1
}
if (-not $ready) { Write-Warning "Vite did not respond in 30s, continuing anyway..." }

try {
    wails dev -frontenddevserverurl http://localhost:5173
} finally {
    if ($viteProcess -and -not $viteProcess.HasExited) {
        Stop-Process -Id $viteProcess.Id -Force -ErrorAction SilentlyContinue
    }
}
