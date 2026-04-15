# Docker Local Compilation Support

Added `docker-compose.local.yml` file, supporting local compilation and multi-architecture deployment.

## New Files

- `docker/docker-composer/docker-compose.local.yml` - Local compilation configuration file

## Compilation Methods

### Default Compilation (AMD64)

```bash
cd docker/docker-composer
docker-compose -f docker-compose.local.yml up --build
```

### ARM64 Compilation (Apple Silicon)

```bash
cd docker/docker-composer
TARGETARCH=arm64 docker-compose -f docker-compose.local.yml up --build
```

## Running Methods

After compilation completes, services will automatically start, including:
- Main server (port 8989)
- Backend management (port 8081)
- Frontend interface (port 8080)
- MySQL database (port 23306)

Access http://<Server IP or Domain>:8080 to view frontend interface.

## 🏗️ Multi-architecture Support

### Automatic Architecture Detection (Recommended)

`docker-compose.local.yml` supports automatic detection of current system architecture:

```bash
# Automatic architecture detection and build (default behavior)
docker-compose -f docker-compose.local.yml up --build
```

### Manual Architecture Specification

If you need to build for specific architecture:

```bash
# Build for ARM64 architecture
TARGETARCH=arm64 docker-compose -f docker-compose.local.yml up --build

# Build for AMD64 architecture
TARGETARCH=amd64 docker-compose -f docker-compose.local.yml up --build
```

### Supported Architectures

- **AMD64/x86_64**: Intel/AMD processors (default)
- **ARM64**: Apple Silicon (M1/M2), ARM servers

## 📁 Configuration File Description

### docker-compose.yml

Uses pre-built official images, suitable for production environment:

```yaml
services:
  mysql:
    image: docker.jsdelivr.fyi/mysql:8.0
  main-server:
    image: docker.jsdelivr.fyi/hackers365/xiaozhi_golang:0.1
  backend:
    image: docker.jsdelivr.fyi/hackers365/xiaozhi_backend:0.1
  frontend:
    image: docker.jsdelivr.fyi/hackers365/xiaozhi_frontend:0.1
```

### docker-compose.local.yml

Local build version, supports code modification and multi-architecture:

```yaml
services:
  main-server:
    build:
      context: ../..
      dockerfile: docker/Dockerfile.main
      args:
        TARGETARCH: ${TARGETARCH:-amd64}
```

## 🔧 Environment Variable Configuration

### Architecture Related

| Variable Name | Default | Description |
|-------|-------|------|
| `TARGETARCH` | `amd64` | Target architecture (amd64/arm64) |


## 🛠️ Common Operations

### View Service Status

```bash
# View all service status
docker-compose ps

# View service logs
docker-compose logs -f main-server
docker-compose logs -f backend
docker-compose logs -f frontend
```

### Stop and Restart Services

```bash
# Stop all services
docker-compose down

# Restart specific service
docker-compose restart main-server

# Rebuild and start
docker-compose up --build
```
