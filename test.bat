@echo == Running tests ===
go test ./...
if errorlevel 1 (
    echo.
    echo TESTS FAILED
    pause
    exit /b 1
)

echo.
echo === Building miniskin.exe ===
go build -o miniskin.exe ./cmd/miniskin
if errorlevel 1 (
    echo.
    echo BUILD FAILED
    pause
    exit /b 1
)

echo.
echo OK: miniskin.exe generated
pause
 