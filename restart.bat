@echo off
:loop
echo Starting LRCleaner...
go run main.go
echo LRCleaner stopped. Restarting in 2 seconds...
timeout /t 2 /nobreak > nul
goto loop

