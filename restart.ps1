# Auto-restart script for LRCleaner
Write-Host "Starting LRCleaner Auto-Restart..." -ForegroundColor Green

while ($true) {
    try {
        Write-Host "`n[$(Get-Date)] Starting LRCleaner..." -ForegroundColor Yellow
        go run main.go
        
        if ($LASTEXITCODE -ne 0) {
            Write-Host "LRCleaner exited with error code: $LASTEXITCODE" -ForegroundColor Red
        } else {
            Write-Host "LRCleaner stopped normally." -ForegroundColor Yellow
        }
    }
    catch {
        Write-Host "Error running LRCleaner: $($_.Exception.Message)" -ForegroundColor Red
    }
    
    Write-Host "Restarting in 3 seconds... (Press Ctrl+C to stop)" -ForegroundColor Cyan
    Start-Sleep -Seconds 3
}

