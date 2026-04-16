@echo off
setlocal enabledelayedexpansion
echo === Xiaozhi Management System Backend Startup Script ===

REM Check parameters
if "%1"=="help" goto :help
if "%1"=="-h" goto :help
if "%1"=="--help" goto :help

REM Set configuration file path
set CONFIG_FILE=manager/backend/config/config.json
set RESET_DB=

if "%1"=="dev" (
    set CONFIG_FILE=manager/backend/config/config.dev.json
    echo Using development environment configuration: %CONFIG_FILE%
) else if "%1"=="prod" (
    set CONFIG_FILE=manager/backend/config/config.prod.json
    echo Using production environment configuration: %CONFIG_FILE%
) else if "%1"=="reset" (
    set RESET_DB=-reset-db
    echo Resetting database and using default configuration: %CONFIG_FILE%
) else if "%1"=="reset-dev" (
    set CONFIG_FILE=manager/backend/config/config.dev.json
    set RESET_DB=-reset-db
    echo Resetting database and using development environment configuration: %CONFIG_FILE%
) else if "%1"=="custom" (
    if "%2"=="" (
        echo Error: Please specify configuration file path
        echo Usage: start.bat custom config.json
        pause
        exit /b 1
    )
    set CONFIG_FILE=%2
    echo Using custom configuration: %CONFIG_FILE%
) else if "%1"=="" (
    echo Using default configuration: %CONFIG_FILE%
) else (
    echo Unknown parameter: %1
    echo Use 'start.bat help' to view help
    pause
    exit /b 1
)

REM Check if configuration file exists
if not exist "%CONFIG_FILE%" (
    echo Error: Configuration file does not exist: %CONFIG_FILE%
    pause
    exit /b 1
)

REM Enter backend directory
cd manager\backend

REM Install dependencies
echo Installing Go dependencies...
go mod tidy

REM Start service
echo Starting service...
if not "%RESET_DB%"=="" (
    echo Warning: Database will be reset, all data will be deleted!
    set /p confirm=Are you sure you want to continue? (y/N): 
    if not "!confirm!"=="y" if not "!confirm!"=="Y" (
        echo Operation cancelled
        pause
        exit /b 0
    )
    go run main.go -config="..\..\%CONFIG_FILE%" %RESET_DB%
) else (
    go run main.go -config="..\..\%CONFIG_FILE%"
)
goto :end

:help
echo Usage:
echo   start.bat                    # Use default configuration file
echo   start.bat dev                # Use development environment configuration
echo   start.bat prod               # Use production environment configuration
echo   start.bat custom config.json # Use custom configuration file
echo   start.bat reset              # Reset database and use default configuration
echo   start.bat reset-dev          # Reset database and use development environment configuration
echo   start.bat help               # Display help information

:end
pause