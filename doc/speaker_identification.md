# Speaker Identification Feature Documentation

> Speaker Identification is a core feature in the xiaozhi-esp32-server-golang project, used to identify the user on the device side and dynamically switch TTS voices based on the identification result.

---

## 1. Feature Overview

Speaker identification extracts voiceprint features (embeddings) from user speech and compares them with pre-registered voiceprint data to achieve user identity recognition.

### Core Capabilities

| Capability | Description |
|------|------|
| 🎤 **Voiceprint Registration** | Upload user audio samples, extract voiceprint features and store them |
| 🔍 **Speaker Identification** | Real-time identification of speaker identity |
| ✅ **Voiceprint Verification** | Verify if audio belongs to a specified user |
| 📡 **Streaming Identification** | Real-time streaming speaker identification via WebSocket |
| 🔊 **Dynamic TTS Switching** | Dynamically switch TTS voices based on identification results |

---

## 2. System Architecture

### 2.1 Overall Architecture

```
┌──────────────────┐     ┌──────────────────────┐     ┌──────────────────┐
│   ESP32 Device   │────▶│ xiaozhi-esp32-server │────▶│   voice-server   │
│  (Audio Capture) │     │     (Main Service)   │     │ (Speaker ID Svc) │
└──────────────────┘     └──────────────────────┘     └──────────────────┘
                                                               │
                                                               ▼
                                                       ┌──────────────────┐
                                                       │  Qdrant Vector DB│
                                                       │ (Store Voiceprints)│
                                                       └──────────────────┘
```

### 2.2 Component Description

| Component | Responsibility |
|------|------|
| **xiaozhi-esp32-server** | Main service, responsible for device connection, session management, speaker identification result processing |
| **voice-server (asr_server)** | Speaker identification service, responsible for feature extraction, registration, identification, verification |
| **Manager (Backend Management)** | Web management backend, provides speaker group management, sample management API and UI |
| **Qdrant** | Vector database, stores voiceprint feature vectors |

---

## 3. Complete Process Description

### 3.1 Voiceprint Registration Process

```
User uploads audio → Manager API → voice-server registration interface → Extract embedding → Store in Qdrant
                  │
                  ▼
            Save to local file + Database record
```

**Detailed Steps:**

1. User uploads audio file (WAV format) in Manager Web interface
2. Manager backend generates unique UUID, saves audio file to local storage
3. Calls voice-server's `/api/v1/speaker/register` interface
4. voice-server uses sherpa-onnx model to extract voiceprint features (192-dimensional vector)
5. Voiceprint features are stored in Qdrant vector database
6. Manager creates `SpeakerSample` database record

### 3.2 Real-time Speaker Identification Process

```
ESP32 captures audio → VAD detects speech → Simultaneously sends to ASR and speaker identification
                                        │
                                        ▼
                              WebSocket streaming identification
                                        │
                                        ▼
                              Get identification result when speech ends
                                        │
                                        ▼
                              Switch TTS voice based on identification result
```

**Detailed Steps:**

1. **VAD Detection**: Audio captured by ESP32 undergoes VAD (Voice Activity Detection)
2. **Dual-channel Sending**: When speech is detected, audio data is simultaneously sent to:
   - ASR service (speech-to-text)
   - Speaker identification service (WebSocket streaming identification)
3. **Streaming Processing**: Speaker identification service continuously receives audio chunks
4. **Result Acquisition**: When speech end (silence) is detected, call `FinishAndIdentify` to get identification result
5. **TTS Switching**: Dynamically switch TTS voice according to the corresponding user's configuration based on identification result

### 3.3 Enable Conditions

Speaker identification will only start when the following conditions are met simultaneously:

- `voice_identify.enable = true`: Speaker identification enabled in global configuration
- Device configuration contains speaker group configuration
- `speakerManager` has been successfully initialized

---

## 4. Configuration Instructions

### 4.1 Main Program Configuration (config.yaml)

Add the following configuration in `config.yaml`:

```yaml
# Speaker identification configuration
voice_identify:
  enable: true                              # Whether to enable speaker identification
  base_url: "http://voice-server:8080"      # voice-server service address
  threshold: 0.6                            # Speaker identification threshold, range 0.0-1.0
```

| Configuration Item | Type | Default | Description |
|--------|------|--------|------|
| `enable` | bool | false | Whether to enable speaker identification feature |
| `base_url` | string | - | HTTP address of voice-server service |
| `threshold` | float | 0.6 | Identification threshold, higher value requires stricter matching |

### 4.2 Docker Compose Configuration

#### Backend Service Environment Variables

```yaml
backend:
  environment:
    - SPEAKER_SERVICE_URL=http://voice-server:8080
```

#### voice-server Service Environment Variables

```yaml
voice-server:
  environment:
    - VAD_ASR_SPEAKER_ENABLED=true
    - VAD_ASR_SPEAKER_VECTOR_DB_HOST=qdrant
    - VAD_ASR_SPEAKER_VECTOR_DB_PORT=6334
    - VAD_ASR_SPEAKER_VECTOR_DB_COLLECTION_NAME=speaker_embeddings
    - VAD_ASR_SPEAKER_THRESHOLD=0.6
    - VAD_ASR_LOGGING_LEVEL=info
```

| Environment Variable | Description |
|----------|------|
| `VAD_ASR_SPEAKER_ENABLED` | Whether to enable speaker identification feature |
| `VAD_ASR_SPEAKER_VECTOR_DB_HOST` | Qdrant service address |
| `VAD_ASR_SPEAKER_VECTOR_DB_PORT` | Qdrant gRPC port |
| `VAD_ASR_SPEAKER_VECTOR_DB_COLLECTION_NAME` | Qdrant Collection name |
| `VAD_ASR_SPEAKER_THRESHOLD` | Speaker identification threshold |
| `VAD_ASR_LOGGING_LEVEL` | Log level |

---

## 5. API Documentation

### 5.1 Manager Backend API

#### Speaker Group Management

| Method | Path | Description |
|------|------|------|
| POST | `/api/speaker-groups` | Create speaker group |
| GET | `/api/speaker-groups` | Get speaker group list |
| GET | `/api/speaker-groups/:id` | Get speaker group details |
| PUT | `/api/speaker-groups/:id` | Update speaker group |
| DELETE | `/api/speaker-groups/:id` | Delete speaker group |
| POST | `/api/speaker-groups/:id/verify` | Verify speaker |

#### Speaker Sample Management

| Method | Path | Description |
|------|------|------|
| POST | `/api/speaker-groups/:id/samples` | Add speaker sample |
| GET | `/api/speaker-groups/:id/samples` | Get sample list |
| GET | `/api/speaker-samples/:id/audio` | Get sample audio file |
| DELETE | `/api/speaker-samples/:id` | Delete sample |

### 5.2 voice-server API

#### HTTP Interfaces

| Method | Path | Description |
|------|------|------|
| POST | `/api/v1/speaker/register` | Register speaker |
| POST | `/api/v1/speaker/identify` | Identify speaker |
| POST | `/api/v1/speaker/verify` | Verify speaker |
| GET | `/api/v1/speaker/list` | Get all speakers |
| DELETE | `/api/v1/speaker/:id` | Delete speaker |
| GET | `/api/v1/speaker/stats` | Get statistics |

#### WebSocket Streaming Identification

**Connection Address:** `ws://voice-server:8080/api/v1/speaker/stream`

**Message Flow:**

1. Client sends audio chunks (PCM float32, little-endian)
2. Client sends completion command: `{"action": "finish"}`
3. Server returns identification result

---

## 6. Vector Database (Qdrant)

### 6.1 Data Storage Structure

```json
{
    "uid": "User ID",
    "agent_id": "Agent ID",
    "speaker_id": "Speaker ID (speaker group primary key)",
    "speaker_name": "Speaker Name (speaker group name)",
    "uuid": "Unique identifier for sample",
    "sample_index": 0,
    "created_at": 1704672000,
    "updated_at": 1704672000
}
```

### 6.2 Vector Configuration

| Configuration | Value |
|------|-----|
| Vector Dimension | 192 |
| Distance Metric | Cosine (Cosine Similarity) |
| Collection Name | `speaker_embeddings` (configurable) |

### 6.3 Data Isolation

Supports multi-dimensional data isolation:

- **UID**: User-level isolation
- **Agent ID**: Agent-level isolation
- Different agents of the same user can have independent speaker data

---

## 7. Database Table Structure

### 7.1 SpeakerGroup (Speaker Group Table)

```sql
CREATE TABLE `speaker_groups` (
  `id` INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `user_id` INT UNSIGNED NOT NULL COMMENT 'Owner User ID',
  `agent_id` INT UNSIGNED NOT NULL COMMENT 'Associated Agent ID',
  `name` VARCHAR(100) NOT NULL COMMENT 'Speaker Name',
  `prompt` TEXT COMMENT 'Role Prompt',
  `description` TEXT COMMENT 'Description',
  `tts_config_id` VARCHAR(100) COMMENT 'TTS Configuration ID',
  `voice` VARCHAR(200) COMMENT 'Voice Value',
  `status` VARCHAR(20) NOT NULL DEFAULT 'active',
  `sample_count` INT NOT NULL DEFAULT 0 COMMENT 'Sample Count',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
);
```

### 7.2 SpeakerSample (Speaker Sample Table)

```sql
CREATE TABLE `speaker_samples` (
  `id` INT UNSIGNED NOT NULL AUTO_INCREMENT,
  `speaker_group_id` INT UNSIGNED NOT NULL COMMENT 'Associated Speaker Group ID',
  `user_id` INT UNSIGNED NOT NULL COMMENT 'Owner User ID',
  `uuid` VARCHAR(36) NOT NULL COMMENT 'UUID Unique Identifier',
  `file_path` VARCHAR(500) NOT NULL COMMENT 'Audio File Local Storage Path',
  `file_name` VARCHAR(255) COMMENT 'Original Filename',
  `file_size` BIGINT COMMENT 'File Size (bytes)',
  `duration` FLOAT COMMENT 'Audio Duration (seconds)',
  `status` VARCHAR(20) NOT NULL DEFAULT 'active',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE INDEX `idx_uuid` (`uuid`)
);
```

---

## 8. User Guide

### 8.1 Deploy voice-server

Refer to the complete deployment configuration in [docker_compose.md](docker_compose.md), ensure the following services are started:

- **Qdrant**: Vector database
- **voice-server**: Speaker identification service

### 8.2 Configure Main Program

Add speaker identification configuration in main program's `config.yaml`:

```yaml
voice_identify:
  enable: true
  base_url: "http://voice-server:8080"
  threshold: 0.6
```

### 8.3 Create Speaker Group

1. Log in to Manager Web console
2. Go to "Agents" → Select target agent → "Speaker Management"
3. Click "New Speaker Group", fill in name, description and other information
4. Configure corresponding TTS voice (optional)

### 8.4 Upload Speaker Sample

1. Click "Add Sample" on speaker group details page
2. Upload WAV format audio file (recommended 3-10 seconds of clear speech)
3. System automatically extracts voiceprint features and stores them

### 8.5 Test Speaker Identification

1. Click "Verify" on speaker group details page
2. Upload test audio
3. View identification result and confidence score

---

## 9. Key Technical Points

### 9.1 Voiceprint Feature Extraction

- Use **sherpa-onnx** model to extract voiceprint features
- Output 192-dimensional embedding vector
- Supports arbitrary sample rate input, automatic resampling

### 9.2 Similarity Calculation

- Use **Cosine Similarity** to calculate voiceprint matching degree
- Similarity range: [-1, 1]
- Default threshold 0.6, can be adjusted according to actual scenarios

### 9.3 VAD Preprocessing

- Use TEN-VAD for silence filtering
- Retain 100ms silence boundaries before and after during registration
- Only send audio segments detected by voice activity during real-time identification

---

## 10. FAQ

### Q1: Speaker identification not working?

Check the following configurations:
1. Whether `voice_identify.enable` is `true`
2. Whether `voice_identify.base_url` is correct
3. Whether device has speaker group configured
4. Whether voice-server service is running normally

### Q2: Low identification accuracy?

- Improve speaker sample quality (clear, no noise, 3-10 seconds)
- Increase number of speaker samples (recommended 3-5 samples)
- Adjust identification threshold

### Q3: TTS voice not switched?

Check if `tts_config_id` or `voice` field is correctly configured in speaker group configuration.

---

## 11. Related Documents

- [Docker Compose Deployment](docker_compose.md)
- [Configuration Documentation](config.md)
- [Vision Recognition](vision.md)
