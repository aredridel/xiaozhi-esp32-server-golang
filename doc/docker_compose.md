# Docker Compose Deployment Guide

## Overview

This project uses Docker Compose for containerized deployment, containing the following core services:

- **MySQL Database Service**: Data storage
- **Main Program Service**: Core business logic
- **Backend Management Service**: API interface service
- **Frontend Management Service**: Web management interface

## Quick Guide (Supplement)

This section is a supplement to `doc/docker.md`, helping to quickly select and implement deployment methods.

### 1. Select Deployment Method

- Recommended: Docker Compose (includes management backend and complete services)
- Simplified: Single container Docker (no console or streamlined mode)

### 2. Docker Compose Quick Path

1) Pull code or prepare `docker-compose.yml`
2) Refer to subsequent "Configuration File Preparation" and "Start Service" in this document to complete configuration
3) Start:

```bash
docker compose up -d
```

4) Management backend default address: `http://<Server IP or Domain>:8080/`

### 3. Single Container Docker (Supplement)

After building or pulling the image according to `doc/docker.md`, run. Common suggestions:

- Map `config/`, `logs/`, `storage/` directories as data volumes
- Expose WebSocket / MQTT / UDP ports externally
- Enable corresponding parameters or use Compose when management backend is needed

### 4. Configuration Wizard and Testing

After startup, you can use the configuration wizard in the management backend to complete engine configuration, and use testing tools for VAD/ASR/LLM/TTS availability and latency testing, as well as OTA full process verification.

### 5. FAQ

- Port conflict: Check 8080/8989/2883/8990 occupancy
- Configuration not taking effect: Confirm data volume mount path is correct, restart container to take effect
- Permission issues: On Linux, pay attention to mount directory permissions and SELinux restrictions

## Service Architecture

### 1. MySQL Database Service (xiaozhi-mysql)

**Configuration Information:**
- Image: `docker.jsdelivr.fyi/mysql:8.0`
- Port mapping: `23306:3306`
- Database name: `xiaozhi_admin`
- Username: `root`
- Password: `password`

**Features:**
- Uses MySQL 8.0
- Configures health check
- Data persistence

### 2. Main Program Service (xiaozhi-main-server)

**Configuration Information:**
- Image: `docker.jsdelivr.fyi/hackers365/xiaozhi_server:0.5`
- Port mapping:
  - `8989:8989` - WebSocket service
  - `2882:2883` - MQTT service
  - `8888:8888/udp` - UDP service

**Dependencies:**
- Depends on MySQL service health status
- Depends on backend service startup completion

**Configuration File Support:**
- Import custom configuration files through volume mount
- Configuration file path: `../../config:/workspace/config`

**ten_vad Support:**
- Docker image already includes ten_vad library (`/workspace/lib/ten-vad/`)
- Runtime library path automatically configured through `LD_LIBRARY_PATH`

### 3. Backend Management Service (xiaozhi-backend)

**Configuration Information:**
- Image: `docker.jsdelivr.fyi/hackers365/xiaozhi_manager_backend:0.5`
- Port mapping: `8081:8080`

**Functions:**
- Provides RESTful API
- Device and user management

**Configuration File Support:**
- Import custom configuration files through volume mount
- Configuration file path: `../../manager/backend/config:/root/config`

### 4. Frontend Management Service (xiaozhi-frontend)

**Configuration Information:**
- Image: `docker.jsdelivr.fyi/hackers365/xiaozhi_manager_frontend:0.5`
- Port mapping: `8080:80`

**Functions:**
- Web management interface (internal control entry)
- Device status and system configuration management

## Deployment Process

### 1. Environment Preparation

Ensure system has Docker and Docker Compose installed:

```bash
docker --version
docker compose version
```

### 2. Configuration File Preparation

Ensure the following directories and files exist:

```
xiaozhi-esp32-server-golang/
├─ docker/docker-composer/
│  └─ docker-compose.yml
├─ config/
│  ├─ config.yaml
│  ├─ config.json
│  └─ (other configuration files)
├─ logs/
│  └─ (log directory)
└─ manager/backend/config/
   ├─ config.yaml
   └─ (other configuration files)
```

**Configuration File Import Description:**
- Main program configuration file imported through volume mount `../../config:/workspace/config`
- Backend configuration file imported through volume mount `../../manager/backend/config:/root/config`

### 3. Start Service

**Must enter `docker/docker-composer/` directory to execute commands:**

```bash
cd docker/docker-composer/
docker compose up -d

docker compose ps
docker compose logs -f
```

### 4. Service Access

- Frontend management interface: `http://<Server IP or Domain>:8080`
- Backend API: `http://localhost:8081`
- WebSocket: `ws://localhost:8989`
- MQTT: `localhost:2882`
- UDP: `localhost:8888`
- MySQL: `localhost:23306`

## Common Operations

```bash
cd docker/docker-composer/

docker compose ps

docker compose logs

docker compose logs -f main-server

docker compose restart

docker compose down

docker compose down -v

docker compose pull

docker compose up -d
```

## Network Configuration

Project uses custom network `xiaozhi-network`:

- MySQL: `mysql:3306`
- Backend: `backend:8080`
- Frontend: `frontend:80`
- Main program: `main-server:8989` (WebSocket) / `main-server:2883` (MQTT) / `main-server:8888` (UDP)

**Port Mapping Summary:**
- 8080 → Frontend management interface
- 8081 → Backend API
- 8989 → WebSocket
- 2882 → MQTT
- 8888 → UDP
- 23306 → MySQL

## Data Persistence

### MySQL Data

Persisted through Docker volume `mysql_data`, data not lost after container restart.

### Configuration Files

- Main program configuration: `../../config:/workspace/config`
- Backend configuration: `../../manager/backend/config:/root/config`

After modifying configuration, restart corresponding service to take effect:

```bash
cd docker/docker-composer/
docker compose restart main-server

docker compose restart backend
```

### Log Files

- Main program logs: `../../logs:/workspace/logs`

## Configuration File Import Methods

### 1. Main Program Configuration

**Location:**
```
xiaozhi-esp32-server-golang/config/
├─ config.yaml
├─ config.json
├─ mqtt_config.json
└─ (other configuration files)
```

**Import:**
1) Put configuration files in `config/`
2) Automatically mount to container `/workspace/config/` after startup
3) Restart main program service after modification:

```bash
cd docker/docker-composer/
docker compose restart main-server
```

### 2. Backend Management Configuration

**Location:**
```
xiaozhi-esp32-server-golang/manager/backend/config/
├─ config.yaml
└─ (other configuration files)
```

**Import:**
1) Put configuration files in `manager/backend/config/`
2) Automatically mount to container `/root/config/` after startup
3) Restart backend service after modification:

```bash
cd docker/docker-composer/
docker compose restart backend
```

### 3. ten_vad Library Files

**Description:**
- Docker image already includes ten_vad library (`/workspace/lib/ten-vad/`)
- Runtime library path automatically configured through `LD_LIBRARY_PATH`
- No additional mount needed to use ten_vad

## Health Check

MySQL service configures health check:

```yaml
healthcheck:
  test: ["CMD", "mysqladmin", "ping", "-h", "localhost", "-u", "root", "-ppassword"]
  timeout: 20s
  retries: 10
  interval: 10s
  start_period: 30s
```

## Troubleshooting

### 1. Service Startup Failure

```bash
cd docker/docker-composer/

docker compose logs [service name]

# Port occupancy check (Linux)
netstat -tulpn | grep [port]
```

### 2. Database Connection Failure

```bash
cd docker/docker-composer/

docker compose ps mysql

docker compose logs mysql

docker compose exec mysql mysql -u root -ppassword
```

### 3. Network Connection Issues

```bash
cd docker/docker-composer/

docker network ls
docker network inspect xiaozhi-network

docker compose exec main-server ping mysql
```

## Performance Optimization Suggestions

1) Set resource limits for each service in production environment
2) Configure log rotation to avoid oversized logs
3) Regularly backup MySQL data
4) Integrate monitoring system

## Security Notes

1) Modify default database password in production environment
2) Expose ports as needed
3) Configure firewall and access control
4) Use trusted image sources

---

## Next Steps

### Access Management Console

After service startup, visit http://<Server IP or Domain>:8080 to enter management console.

**[Management Console Guide →](manager_console_guide.md)**

### Configure ESP32 Device

Refer to [ESP32 Device Access Guide](esp32_xiaozhi_backend_guide.md) to complete device access.
