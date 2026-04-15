#!/bin/bash

echo "=== Xiaozhi Management System Backend Startup Script ==="

# 检查参数
if [ "$1" = "help" ] || [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    echo "Usage:"
    echo "  ./start.sh                    # Use default configuration file"
    echo "  ./start.sh dev                # Use development environment configuration"
    echo "  ./start.sh prod               # Use production environment configuration"
    echo "  ./start.sh custom config.json # Use custom configuration file"
    echo "  ./start.sh reset              # Reset database and use default configuration"
    echo "  ./start.sh reset-dev          # Reset database and use development environment configuration"
    echo "  ./start.sh help               # Display help information"
    exit 0
fi

# Set configuration file path
CONFIG_FILE="manager/backend/config/config.json"

RESET_DB=""

case "$1" in
    "dev")
        CONFIG_FILE="manager/backend/config/config.dev.json"
        echo "Using development environment configuration: $CONFIG_FILE"
        ;;
    "prod")
        CONFIG_FILE="manager/backend/config/config.prod.json"
        echo "Using production environment configuration: $CONFIG_FILE"
        ;;
    "reset")
        RESET_DB="-reset-db"
        echo "Resetting database and using default configuration: $CONFIG_FILE"
        ;;
    "reset-dev")
        CONFIG_FILE="manager/backend/config/config.dev.json"
        RESET_DB="-reset-db"
        echo "Resetting database and using development environment configuration: $CONFIG_FILE"
        ;;
    "custom")
        if [ -z "$2" ]; then
            echo "Error: Please specify configuration file path"
            echo "Usage: ./start.sh custom config.json"
            exit 1
        fi
        CONFIG_FILE="$2"
        echo "Using custom configuration: $CONFIG_FILE"
        ;;
    "")
        echo "Using default configuration: $CONFIG_FILE"
        ;;
    *)
        echo "Unknown parameter: $1"
        echo "Use './start.sh help' to view help"
        exit 1
        ;;
esac

# Check if configuration file exists
if [ ! -f "$CONFIG_FILE" ]; then
    echo "Error: Configuration file does not exist: $CONFIG_FILE"
    exit 1
fi

# Enter backend directory
cd manager/backend

# Install dependencies
echo "Installing Go dependencies..."
go mod tidy

# Start service
echo "Starting service..."
if [ -n "$RESET_DB" ]; then
    echo "Warning: Database will be reset, all data will be deleted!"
    read -p "Are you sure you want to continue? (y/N): " confirm
    if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
        echo "Operation cancelled"
        exit 0
    fi
    go run main.go -config="../../$CONFIG_FILE" $RESET_DB
else
    go run main.go -config="../../$CONFIG_FILE"
fi