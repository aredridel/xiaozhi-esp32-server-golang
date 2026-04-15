# Voice Clone Feature Documentation

This document describes the Voice Clone feature in the project, including the creation/listening/retry workflow for regular users and quota management for administrators.

Related pages and documents:

- Administrator TTS Configuration Management (provides available TTS configurations for users)
- Administrator User Management -> Clone Quota
- Regular user Voice Clone
- [Management Console Guide](./manager_console_guide.md)

---

## 1. Feature Overview

The Voice Clone feature allows users to upload audio (or record via browser) to create a cloned voice on supported TTS providers, which can then be selected for an agent/character to use for announcements.

Currently supported clone providers (frontend and backend):

- minimax
- cosyvoice
- aliyun_qwen (Qwen)

Providers not in the above list cannot be used for voice cloning, even if they support regular TTS synthesis.

---

## 2. Roles and Permissions

### 2.1 Regular Users

Can:

- Create cloned voices
- View clone task status
- Listen to original and cloned audio
- Edit clone names
- Retry failed tasks

### 2.2 Administrators

Can:

- Configure and enable clone-capable TTS providers
- Set clone quotas per user by `TTS Configuration` (optional)

---

## 3. Prerequisites

Before using, please confirm:

1. Administrator has created and enabled at least one TTS configuration (provider is `minimax` / `cosyvoice` / `aliyun_qwen`)
2. Regular users can see this TTS configuration on the "Voice Clone" page
3. (Optional) Administrator has assigned clone quota to the user

Note:

- If no quota is configured, it defaults to historical behavior, typically treated as "unlimited"

---

## 4. Regular User Workflow

Entry point:

- Regular User -> Voice Clone

## 4.1 Creating a Cloned Voice

Click Create Cloned Voice and fill in:

- Clone Name (optional, will use filename if not provided)
- TTS Configuration (must select a configuration that supports cloning)
- Audio Source (upload audio / browser recording)
- Audio Transcript (required or not depends on provider capabilities)
- Text Language (e.g., zh-CN / en-US)

After submission, two results may occur:

- Immediate success (rare)
- Clone task submitted, processing in background (common, asynchronous)

## 4.2 Viewing Task Status

The list displays:

- Provider
- Associated TTS Configuration
- Cloned Voice ID
- Task Status
- Failure reason (if any)
- Creation time

Common status types:

- Queuing / Processing
- Completed (can listen)
- Failed (can view reason and retry)

## 4.3 Listening and Management

Each clone record supports the following operations:

- Original Audio: Play the audio sample submitted by the user
- Preview Clone: Play the cloned voice returned by the provider (only shown for successful status)
- Edit: Modify the clone name
- Re-clone: Resubmit failed tasks (only shown for failed status)

---

## 5. Provider Differences and Notes

## 5.1 Minimax

Frontend and backend will validate audio constraints. Common rules:

- Audio format typically requires WAV
- Audio duration should be at least 10 seconds

The page will show prompts in the upload/recording area and prevent submission if duration is insufficient.

## 5.2 CosyVoice

Features:

- Supports cloning
- Commonly requires filling in "Audio Transcript" (returned by provider capability interface)

Whether it's actually required depends on the current provider capability prompt on the page.

## 5.3 Qwen (`aliyun_qwen`)

Features:

- Supports cloning
- Supports more audio formats (e.g., WAV/MP3/M4A, check page prompts)
- After selecting this cloned voice, the runtime will automatically switch to the corresponding clone model (frontend will show a prompt)

---

## 6. Clone Quota Management (Administrator)

Entry point:

- Administrator -> User Management -> Clone Quota

Administrators can configure clone quotas for a regular user by `TTS Configuration ID`:

- -1: Unlimited
- 0: Creation prohibited
- Positive integer: Maximum number of clones allowed

Quota statistics typically count by "submitted clone tasks" (failed retries should also be included in the counting strategy, please use according to current business rules).

---

## 7. API Documentation (User Side)

### 7.1 Capability Detection

- GET /user/voice-clone/capabilities?provider=<provider>

Purpose:

- Get whether provider is enabled
- Whether transcript is required
- Text length range
- Supported language list

### 7.2 Clone Records and Task Operations

- POST /user/voice-clones (create clone, multipart/form-data)
- GET /user/voice-clones (list)
- PUT /user/voice-clones/:id (modify name)
- POST /user/voice-clones/:id/retry (retry failed task)
- GET /user/voice-clones/:id/preview (preview cloned voice)

### 7.3 Original Audio Management

- GET /user/voice-clones/:id/audios
- GET /user/voice-clones/audios/:audio_id/file

---

## 8. API Documentation (Administrator Quota)

- GET /admin/users/:id/voice-clone-quotas
- PUT /admin/users/:id/voice-clone-quotas

---

## 9. FAQ and Troubleshooting

### 9.1 No selectable TTS configurations visible on page

Check:

1. Whether administrator has enabled TTS configuration
2. Whether TTS provider is in the supported clone list (`minimax/cosyvoice/aliyun_qwen`)
3. Whether current user has permission to access this configuration

### 9.2 Submission shows "This provider requires audio transcript"

This means the provider capability requires transcript to be filled in. Please add the audio transcript and resubmit.

### 9.3 Submission shows insufficient quota

Administrator needs to increase quota or set to -1 for the corresponding TTS Configuration ID in User Management -> Clone Quota.

### 9.4 Clone successful but cannot preview

Check:

1. Whether task status is completed
2. Whether provider preview interface is normal
3. Whether browser blocked audio autoplay (manually click play button to retry)

---

## 10. Usage Recommendations

- Prepare independent TTS configurations for each scenario (for easier quota control and billing attribution)
- Submit audio using clean human voice in low-noise environments
- Ensure transcript matches audio content as closely as possible for better clone quality and stability
