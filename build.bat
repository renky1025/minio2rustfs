@echo off
:: Build script for Windows - minio2rustfs migration tool

setlocal enabledelayedexpansion

set BINARY_NAME=minio2rustfs
set MAIN_PATH=.\cmd\main.go
set BUILD_DIR=build
set VERSION=dev
set BUILD_TIME=%date:~0,4%-%date:~5,2%-%date:~8,2%T%time:~0,2%:%time:~3,2%:%time:~6,2%Z

if "%1"=="" goto help
if "%1"=="help" goto help
if "%1"=="build" goto build
if "%1"=="clean" goto clean
if "%1"=="test" goto test
if "%1"=="fmt" goto fmt
if "%1"=="vet" goto vet
if "%1"=="run" goto run
if "%1"=="deps" goto deps
if "%1"=="all" goto all
goto help

:help
echo Available commands:
echo   build   - Build the binary
echo   clean   - Clean build artifacts
echo   test    - Run tests
echo   fmt     - Format code
echo   vet     - Vet code
echo   run     - Build and run the application
echo   deps    - Download dependencies
echo   all     - Clean, format, vet, test, and build
echo.
echo Example usage:
echo   build.bat build
echo   build.bat test
goto end

:build
echo Building %BINARY_NAME%...
if not exist %BUILD_DIR% mkdir %BUILD_DIR%
go build -o %BUILD_DIR%\%BINARY_NAME%.exe %MAIN_PATH%
if %errorlevel%==0 (
    echo Build completed: %BUILD_DIR%\%BINARY_NAME%.exe
) else (
    echo Build failed!
    exit /b 1
)
goto end

:clean
echo Cleaning...
go clean
if exist %BUILD_DIR% rmdir /s /q %BUILD_DIR%
echo Clean completed
goto end

:test
echo Running tests...
go test -v ./...
goto end

:fmt
echo Formatting code...
go fmt ./...
goto end

:vet
echo Vetting code...
go vet ./...
goto end

:run
call :build
if %errorlevel%==0 (
    echo Running %BINARY_NAME%...
    %BUILD_DIR%\%BINARY_NAME%.exe --help
)
goto end

:deps
echo Downloading dependencies...
go mod download
go mod tidy
echo Dependencies updated
goto end

:all
call :clean
call :fmt
call :vet
call :test
call :build
goto end

:end
endlocal