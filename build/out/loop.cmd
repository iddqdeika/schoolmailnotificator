for /l %%q in (0) do (
schoolnotificator.exe >> log.log
ping 192.0.2.2 -n 1 -w 10000 > nul
)
pause