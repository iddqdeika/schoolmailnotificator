rmdir /Q /S out
md out
go build -o out\\schoolnotificator.exe ..\main.go
copy ..\cfg.json out\cfg.json
copy .\loop.cmd out\loop.cmd
pause