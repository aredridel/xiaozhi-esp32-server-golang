# asr_server Merge Solution (Same Form as manager/backend)

## Goal

- **asr_server maintains independent repository form**: Has its own `go.mod`, `main.go`, can be cloned, built, and run separately.
- **Main program can initialize**: Like `manager/backend`, the main process references the subdirectory through `replace`, and starts asr_server's HTTP service within the process on demand (independent port), without needing to start a separate process.

## Introduction Method: Recommend Using Git Submodule

The main repository can obtain the `asr_server/` directory in two ways:

| Method | Description |
|------|------|
| **Git Submodule (Recommended)** | asr_server maintains independent Git repository; main repository uses `git submodule add` to reference, getting a "directory pointing to a specific commit of asr_server", main repository only records submodule path and commit number. |
| Copy/Move Code | Put asr_server code directly into main repository directory, asr_server and main repository share a Git history (or as part of main repository). |

The following explanation uses **Submodule** method as the standard; main repository side `replace` and embedded startup logic are the same as "copy method".

## Reference: manager/backend Merge Form

| Item | manager/backend Practice |
|--------|----------------------|
| Directory | `manager/backend/` under main repository |
| Module Name | `xiaozhi/manager/backend` (backend internal go.mod) |
| Main Repository Reference | `replace xiaozhi/manager/backend => ./manager/backend` |
| Independent Run | `manager/backend/main.go`: LoadWithPath → database.Init → router.Setup → r.Run() |
| Main Program Embedding | `cmd/server/manager_http.go`: Same set of config/database/router, start `http.Server` on another port |

## asr_server Merge Design (Aligned with Above Form)

### 1. Directory and Module (Submodule Method)

- **asr_server must first have independent Git repository** (if currently in monorepo, can first split into independent repo, or use existing asr_server repository URL).
- **Add submodule in main repository** (execute in main repository root directory, and `asr_server` directory does not exist yet):
  ```bash
  cd xiaozhi-esp32-server-golang
  git submodule add <asr_server repository URL> asr_server
  ```
  After completion, main repository will have:
  - Directory `asr_server/` (content is the current checkout of a commit from asr_server repository)
  - File `.gitmodules`, and submodule records visible in `git submodule status`
- **Directory Path**: In main repository is `xiaozhi-esp32-server-golang/asr_server/`, consistent with "copy method", main repository Go code and go.mod `replace` both point to `./asr_server`.
- **Module Name**: Keep asr_server existing module name **`voice_server`** (convenient for it to `go build` directly as independent repository, without changing import).
- **Main Repository go.mod**: Add one line:
  - `replace voice_server => ./asr_server`
- **asr_server go.mod**: Keep `module voice_server`, do not reference main repository; when independent repository has no replace, after merging into main repository only main repository side needs replace.

**Clone main repository and pull submodule** (choose one):

```bash
# Clone and pull submodule at once
git clone --recurse-submodules <main repository URL>

# Or clone first then initialize submodule
git clone <main repository URL>
cd xiaozhi-esp32-server-golang
git submodule update --init --recursive
```

**CI / Automated Build**: If main repository needs to build code depending on asr_server, need to execute `git submodule update --init --recursive` before building (or use `--recurse-submodules` clone).

### 2. Independent Run (asr_server is still "independent repository")

- When cloning/opening `asr_server` directory separately:
  - `go build -o asr_server .`
  - `./asr_server` uses `config.json` (or `-config` specifies path), behavior consistent with current.
- Does not depend on main repository; main repository's `replace` only affects main repository build.

### 3. Main Program Initialization (Embed asr_server)

- **Entry**: Add `cmd/server/asr_server_http.go` in main repository (same level as `manager_http.go`).
- **Logic** (consistent with manager_http):
  1. Called by main process at startup according to configuration (e.g., `-asr-enable` + `-asr-config`).
  2. Use asr_server packages:
     - `voice_server/config`: `InitConfig(configPath)`, then `GetConfig()` to get `*Config`.
     - `voice_server/internal/bootstrap`: `InitApp(cfg)` to get `*AppDependencies`.
     - `voice_server/internal/router`: `NewRouter(deps)` to get `*gin.Engine`.
  3. Use `deps.RateLimiter.Middleware(r)` as Handler, start `http.Server` on **separate port** (e.g., 8080), `ListenAndServe` in goroutine.
  4. Provide `StopAsrServerHTTP()` on exit, do `Shutdown` on `http.Server`, and do necessary resource release (such as components needing Close in bootstrap).
- **Configuration**: asr_server still uses its own `config.json`; when embedded, configuration file path is specified by main process parameter or main repository configuration (e.g., `asr_server/config.json` or `config/asr_server.json`).

### 4. Main Repository Change List (Submodule Method)

| Location | Change |
|------|------|
| Main repository root | Execute `git submodule add <asr_server repository URL> asr_server`, get `asr_server/` directory and `.gitmodules` (asr_server needs to have independent Git repository first) |
| `xiaozhi-esp32-server-golang/go.mod` | Add `replace voice_server => ./asr_server`; if main repository code needs to import voice_server, add `voice_server` in `require` (or automatically added by `go mod tidy`) |
| `xiaozhi-esp32-server-golang/cmd/server/main.go` | Parse `-asr-enable`, `-asr-config`; if enable, call `StartAsrServerHTTP(configPath)` before `Run()`; call `StopAsrServerHTTP()` after `<-quit` |
| New `xiaozhi-esp32-server-golang/cmd/server/asr_server_http.go` | Implement `StartAsrServerHTTP(configPath string)`, `StopAsrServerHTTP()`, internally use `voice_server/config`, `voice_server/internal/bootstrap`, `voice_server/internal/router`, consistent with manager_http pattern |

### 5. asr_server Side Needs to Cooperate with Exposure

- **config**: Already has `InitConfig(path)`, `GetConfig()`, main process can use directly.
- **bootstrap**: Already has `InitApp(cfg *config.Config)`, returns `*AppDependencies`, main process can use directly.
- **router**: Already has `NewRouter(deps) *gin.Engine`; main process uses `deps.RateLimiter.Middleware(r)` as Handler.
- **Graceful Exit**: If there are resources needing `Close()` in bootstrap (such as VAD pool, global recognizer, etc.), need to provide unified `Shutdown(deps *AppDependencies)` or similar function in asr_server for `StopAsrServerHTTP()` to call; if not currently available, can first only do `Server.Shutdown`, add later.

### 6. Dependencies and Build

- asr_server dependencies (sherpa-onnx, qdrant, ten-vad, etc.) remain in **asr_server/go.mod**; main repository **does not** promote asr_server dependencies to main go.mod require, only references submodule through `require voice_server` (or equivalent), `go mod tidy` pulls dependencies in main repository.
- If main repository build has missing dependencies, then explicitly add direct dependencies used by asr_server in main repository go.mod `require`.
- CGO, local lib (such as ten-vad, sherpa-onnx so/dll) still follow asr_server existing method in asr_server directory or main repository unified `lib/`, explained in build scripts/documentation.

### 7. Differences from manager/backend Description

- manager/backend module name is `xiaozhi/manager/backend`, asr_server keeps `voice_server`, so asr_server as independent repository does not need to change import.
- Main repository uses `replace voice_server => ./asr_server`, no need to change asr_server internal package paths.
- Main program "initialization" method is consistent: do not call asr_server's `main()`, only reuse config + bootstrap + router, start HTTP service with independent port in main process.

### 8. Summary (Submodule Method)

- **Independent Repository**: asr_server is independent Git repository, has its own `go.mod` (`module voice_server`) and `main.go`, can be cloned, built, and run separately.
- **Merge into Main Repository**: Main repository uses **Git submodule** to reference asr_server, gets `asr_server/` directory; main repository `replace voice_server => ./asr_server`; after cloning main repository need to execute `git submodule update --init` (or `git clone --recurse-submodules`).
- **Main Program Initialization**: Main repository adds `asr_server_http.go`, starts asr_server's HTTP service (independent port) in process according to configuration, logic aligned with `manager_http.go`.

**Build Description**: asr_server depends on sherpa-onnx (CGO), main repository makes embedding optional through **build tags**:
- **Default Build** (do not enable embedded asr_server): `go build -o xiaozhi_server ./cmd/server`, at this time `-asr-enable` will print "not compiled into this binary" prompt.
- **Enable Embedded asr_server**: `go build -tags asr_server -o xiaozhi_server ./cmd/server`, requires local machine to have CGO and sherpa-onnx required environment.

If confirm implementation according to this solution, can further detail: asr_server internal `Shutdown(deps)` responsibility list, default port and configuration path, and main repository `main.go` parameter naming and default values.
