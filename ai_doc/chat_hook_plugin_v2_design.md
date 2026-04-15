# Chat Hook / Plugin V2 Design Document

## 1. Document Objectives

This document answers three questions:

1. What are the reasonable boundaries of the current chat pipeline Hook architecture;
2. What capabilities should V2 prioritize;
3. How to evolve in stages while minimizing disruption to existing business main pipelines.

This document is not positioned as "building a complete plugin marketplace", but rather to deliver a **governable, extensible, observable** Chat Hook / Plugin V2 solution for the current repository.

---

## 2. TL;DR

### 2.1 One-Sentence Conclusion

The current implementation already has a usable prototype of a **Chat Hook Framework**, but is not yet suitable to be directly defined as a "complete Plugin Platform".

### 2.2 V2 Core Propositions

The core of V2 is not "rewrite from scratch", but rather:

- Retain existing business integration points;
- Clearly separate **Interceptor (can modify main flow)** from **Observer (observation only)**;
- Add **bounded queue, timeout, drop policy, metrics** for asynchronous execution;
- Introduce **PluginMeta + Registry + Lifecycle**;
- Complete contracts for ASR / LLM / TTS / Metric events.

### 2.3 Recommended Priorities

| Priority | Recommendation | Goal |
| --- | --- | --- |
| P0 | Async Runtime Governance | Prevent slow plugins from dragging down the system |
| P0 | Interceptor / Observer Semantic Separation | Reduce semantic mixing |
| P0 | Payload Contract Documentation | Reduce plugin misuse |
| P1 | PluginMeta / Registry | Make plugin registration manageable |
| P1 | Lifecycle / Config | Support stateful plugins |
| P2 | Multi-queue / Multi-worker / Tracing Integration | Support more complex extensions |

---

## 3. Current Status Assessment

## 3.1 What the Current Design Got Right

The current implementation already has the following advantages:

1. **Correct integration points**
   - Hooks are already connected at ASR final output, LLM input/output, TTS input/output start/end, and Metric stages;
   - These are the most valuable positions in the chat main pipeline.

2. **Layering is basically correct**
   - `internal/pkg/hooks` is responsible for the generic execution framework;
   - `internal/domain/chat/hooks` is responsible for chat domain context, events, and typed payloads;
   - `internal/app/server/chat/*` is only responsible for emitting at appropriate positions.

3. **Typed façade is established**
   - The business side no longer directly operates on `any`;
   - This provides a good foundation for V2 to continue enhancing contracts and governance.

4. **Pluggability is already usable**
   - Currently supports statistics plugins, text rewriting, flow interception, and other built-in capabilities.

## 3.2 Current Main Shortcomings

What most needs to be addressed is not "wrong abstraction", but "insufficient governance capability".

### A. Semantic Mixing

Currently, a unified Hook model carries two completely different types of requirements:

- Interceptors that can modify the main flow;
- metric / audit / telemetry that only observe.

This brings the following problems:

- `stop` semantics are not suitable for all events;
- `payload modification` is only suitable for some stages;
- Plugin authors don't easily understand what each event can do.

### B. Insufficient Async Execution Governance

Current pain points of async execution:

- Queue has no upper limit;
- Missing timeout;
- Missing dropped metrics;
- All async handlers share single-threaded serial consumption;
- Lack of isolation for slow plugins.

### C. Plugin Registration is Hardcoded

Currently mainly registered via `RegisterBuiltinPlugins` in code. While simple, this is not conducive to:

- Viewing which plugins are currently loaded;
- Enabling/disabling switches;
- Configuration by environment;
- Plugin-level debugging.

### D. Unclear Contracts

Although `ASROutputData` / `LLMInputData` / `LLMOutputData` / `TTSInputData` / `MetricData` already exist, the following constraints are still missing:

- Which fields are allowed to be modified;
- Which fields cannot be empty;
- What is the business semantics of `stop`;
- Whether `Err` is allowed to be overridden;
- What is the maximum allowed execution time for plugins.

---

## 4. V2 Positioning and Design Principles

## 4.1 Architecture Positioning

V2 recommends officially naming the system:

> **Chat Interceptor & Observer Framework**

Rather than directly calling it:

> Plugin Platform

This naming is closer to the real capabilities of the current stage and is more conducive to managing team expectations.

## 4.2 Design Principles

V2 should follow these principles:

1. **Keep the business main pipeline stable**
   - Do not massively change existing ASR / LLM / TTS logic;
   - Prioritize enhancements in the Hook Runtime and Domain Facade layers.

2. **Govern first, then platformize**
   - First resolve semantics, boundaries, monitoring, and stability;
   - Then consider more complex plugin ecosystems.

3. **Distinguish Interceptor from Observer first**
   - All behaviors that can modify the main flow must be explicitly classified as Interceptor;
   - All read-only observation behaviors must be explicitly classified as Observer.

4. **Clarify contracts before expanding plugin count**
   - Without contracts, more plugins mean higher maintenance costs.

5. **Incremental evolution, no one-time rewrite**
   - Prioritize designs compatible with existing emit interfaces;
   - Migration should be staged.

---

## 5. V2 Overall Architecture

## 5.1 Three-Layer Structure

V2 continues the current three-layer structure but strengthens responsibility boundaries.

### Layer 1: Business Main Pipeline Layer

Responsibilities:

- Emit at key nodes of ASR / LLM / TTS / Session Metric;
- Do not directly perceive plugin registration, scheduling policies, or lifecycle.

Not responsible for:

- Plugin registration;
- Plugin execution governance;
- Plugin metadata management.

### Layer 2: Chat Domain Hook Layer

Responsibilities:

- Define chat domain events, typed payloads, and domain context;
- Expose a unified and stable entry point to business code;
- Constrain field contracts and stop/error semantics.

### Layer 3: Generic Runtime Layer

Responsibilities:

- Plugin registration;
- Ordering and execution;
- Async scheduling;
- timeout / drop / metrics;
- Lifecycle management.

## 5.2 Hook Roles in Request Path

```text
ASR final text
  -> ASR Output Interceptors
  -> LLM Input Interceptors
  -> LLM Execution
  -> LLM Output Interceptors
  -> TTS Input Interceptors
  -> TTS Output Observers

At the same time:
  Metric Observers observe at turn_start / asr_first / asr_final / llm_start /
  llm_first / llm_end / tts_start / tts_first / tts_stop stages
```

The core of this separation is:

- Business main pipeline only cares about "when to emit";
- Interceptor is responsible for modification;
- Observer is responsible for observation;
- Runtime is responsible for governance.

---

## 6. Event Model Design

## 6.1 Event Layering

### A. Interceptor Events

Used for synchronous modification and flow control.

Recommended to keep:

- `chat.asr.output`
- `chat.llm.input`
- `chat.llm.output`
- `chat.tts.input`

These events should have:

- Ordered execution by priority;
- Modifiable payload;
- Can `stop`;
- Can return error;
- Must return quickly.

### B. Observer Events

Used for observation, instrumentation, logging, tracing, auditing, etc.

Recommended to classify as Observer:

- `chat.metric`
- `chat.tts.output.start`
- `chat.tts.output.stop`
- Future extended audit / trace / debug events

These events should have:

- Read-only by default;
- Cannot stop main flow;
- Do not participate in main flow payload changes;
- Can execute asynchronously;
- Errors only affect observation pipeline.

## 6.2 Event Naming Recommendations

Continue using existing layered naming, not recommended to immediately overhaul the naming system. Recommended to keep:

- `chat.asr.output`
- `chat.llm.input`
- `chat.llm.output`
- `chat.tts.input`
- `chat.tts.output.start`
- `chat.tts.output.stop`
- `chat.metric`

Reasons:

- Current naming is already intuitive;
- Compatible with existing implementation;
- Lowest migration cost.

---

## 7. Runtime Design

## 7.1 PluginMeta

V2 introduces unified metadata for display, enable/disable, diagnostics, and sorting.

```go
package hooks

type PluginKind string

const (
    PluginKindInterceptor PluginKind = "interceptor"
    PluginKindObserver    PluginKind = "observer"
)

type PluginMeta struct {
    Name        string
    Version     string
    Description string
    Priority    int
    Enabled     bool
    Kind        PluginKind
    Stage       string
}
```

### Design Notes

- `Name`: Globally unique identifier;
- `Version`: Facilitates subsequent compatibility and canary releases;
- `Priority`: Sorting basis;
- `Enabled`: Runtime switch;
- `Kind`: Distinguish interceptor / observer;
- `Stage`: Declare the stage to be mounted.

## 7.2 Registry

V2 recommends introducing an explicit registration center, rather than letting Runtime decide "which plugins exist".

```go
package hooks

type Registration struct {
    Meta     PluginMeta
    Register func(*Hub)
}

type Registry interface {
    Add(reg Registration)
    List() []Registration
}
```

### What Registry is Responsible For

- Save plugin definitions;
- Expose enumerable registration list;
- Support filtering by configuration whether enabled;
- Provide basic data for debugging and observation.

### What Registry is NOT Responsible For

- Do not directly execute plugins;
- Do not directly carry business state;
- Do not replace Runtime execution logic.

## 7.3 Lifecycle

Provide minimal lifecycle for stateful plugins.

```go
package hooks

type Lifecycle interface {
    Init(context.Context) error
    Close() error
}
```

Recommendations:

- Stateless plugins may not implement;
- Plugins with cache, background tasks, or connection pools implement this interface;
- Runtime uniformly manages invocation timing.

## 7.4 Interceptor Interface

```go
package hooks

type Interceptor[T any] interface {
    Meta() PluginMeta
    Handle(Context, T) (T, bool, error)
}
```

Design Intent:

- Retain three capabilities: "modification + stop + error";
- Use generics to improve compile-time constraints;
- Reduce misuse risks caused by `any`.

## 7.5 Observer Interface

```go
package hooks

type Observer[T any] interface {
    Meta() PluginMeta
    Handle(Context, T)
}
```

Design Intent:

- Clearly "observe only, do not modify";
- Eliminate misuse of `stop` semantically;
- Facilitate independent scheduling strategies for observers.

## 7.6 Async Runtime

### Current Problems

The biggest problem with the current async execution model is not "it doesn't work", but "lacks boundaries".

### V2 Design Goals

Add the following capabilities for async observers:

- bounded queue;
- timeout;
- dropped statistics;
- per-plugin execution metrics;
- Future extensibility to multi-queue / multi-worker.

### Recommended Configuration

```go
package hooks

type AsyncConfig struct {
    QueueSize    int
    WorkerCount  int
    DropWhenFull bool
    Timeout      time.Duration
}
```

### Recommended Default Values

- `QueueSize = 1024`
- `WorkerCount = 1`
- `DropWhenFull = true`
- `Timeout = 200ms`

### Recommended Strategies

1. Default to single worker first to guarantee ordering semantics;
2. When queue is full, prioritize dropping observer events rather than slowing down main pipeline;
3. Record dropped count and timeout count;
4. If high-load observers appear in the future, split queues by event or plugin.

---

## 8. Domain Contracts

V2 must explicitly document payload contracts.

## 8.1 ASROutputData

Purpose: ASR final text and speaker result modification.

| Field | Modifiable | Description |
| --- | --- | --- |
| `Text` | Yes | Can be cleaned, normalized, filtered |
| `SpeakerResult` | Yes | Can be speaker-corrected or enhanced |

Constraints:

- Plugins must not block for long periods;
- `stop=true` means this round's text will not continue into LLM;
- If empty text is returned, the plugin should bear the consequences.

## 8.2 LLMInputData

Purpose: Modify messages and tools before initiating LLM request.

| Field | Modifiable | Description |
| --- | --- | --- |
| `UserMessage` | Yes | Cannot be empty |
| `RequestMessages` | Yes | Can be trimmed, reordered, inject system prompt |
| `Tools` | Yes | Can be filtered or appended |

Constraints:

- `UserMessage` cannot be `nil`;
- Plugins must ensure output still meets minimum input requirements of downstream LLM Provider;
- `stop=true` means this LLM request is intercepted and terminated.

## 8.3 LLMOutputData

Purpose: After LLM output completes, modify display text or supplement error semantics.

| Field | Modifiable | Description |
| --- | --- | --- |
| `FullText` | Yes | Can be safely rewritten, formatted, speech-friendly |
| `Err` | Caution | Recommend appending context, not directly swallowing underlying errors |

Constraints:

- Not recommended for plugins to silently override underlying real errors;
- `stop=true` means subsequent processing will not continue into TTS or message update;
- In the future, `Err` can be split into `OriginErr` / `DisplayErr`.

## 8.4 TTSInputData

Purpose: Make text speech-readable before entering TTS.

| Field | Modifiable | Description |
| --- | --- | --- |
| `Text` | Yes | Can be made speech-readable for numbers, punctuation, emoji |
| `IsStart` | Default No | Considered protocol boundary field |
| `IsEnd` | Default No | Considered protocol boundary field |

Constraints:

- Regular plugins should only modify `Text`;
- `IsStart` / `IsEnd` should be reserved for higher privilege or dedicated plugins;
- `stop=true` means current segment does not enter TTS.

## 8.5 MetricData

Purpose: Pipeline observation.

| Field | Modifiable | Description |
| --- | --- | --- |
| `Stage` | No | Read-only |
| `Ts` | No | Read-only |
| `Err` | No | Read-only, for observation only |

Constraints:

- Cannot stop main flow;
- Cannot modify and then feedback to main pipeline;
- For logging, metrics, tracing, debugging only.

---

## 9. Configuration Model

Recommend adding minimal configuration model for Hook system:

```yaml
chat_hooks:
  enabled: true
  async:
    queue_size: 1024
    worker_count: 1
    drop_when_full: true
    timeout_ms: 200
  plugins:
    statistic_plugin:
      enabled: true
      priority: 100
```

Configuration design goals:

- Support plugin enable/disable;
- Support priority override;
- Support async runtime parameter control;
- Reserve space for future plugin-level config schema.

---

## 10. Observability Requirements

Runtime should collect at least the following metrics:

- plugin invocation count;
- plugin execution time;
- error count;
- stop count (interceptor);
- dropped count (observer async);
- timeout count;
- current async queue length.

Recommended to include in logs/metrics:

- `plugin_name`
- `plugin_kind`
- `stage`
- `priority`
- `duration_ms`
- `result`

If tracing is added later, can further record:

- session_id
- device_id
- turn_id
- correlation_id

---

## 11. Migration Plan

## 11.1 Phase 1: Runtime Enhancement (P0)

Goal: Improve stability without changing business integration points.

Work items:

- Add bounded queue for async observer;
- Add timeout and dropped statistics;
- Add Runtime basic metrics;
- Maintain compatibility with existing `Emit` / `RegisterSync` / `RegisterAsync` interfaces.

Deliverables:

- Stability improvement;
- Provide boundaries for subsequent observer expansion.

## 11.2 Phase 2: Semantic Layering (P0)

Goal: Explicitly distinguish Interceptor from Observer.

Work items:

- Add clear façade in Domain Hook layer;
- Convert `Metric` events to standard observer semantics;
- Clearly define the set of events that cannot stop.

Deliverables:

- Clearer semantics;
- Plugin authors less likely to misuse.

## 11.3 Phase 3: Registry + Meta (P1)

Goal: Extract "which plugins exist" from hardcoded logic.

Work items:

- Introduce `PluginMeta`;
- Introduce `Registration` / `Registry`;
- Support enable/disable plugins by configuration;
- Support listing currently loaded plugins.

Deliverables:

- Transparent registration;
- Facilitates debugging, observation, and configuration governance.

## 11.4 Phase 4: Contracts and Lifecycle (P1)

Goal: Formalize plugin boundaries.

Work items:

- Solidify payload contracts;
- Introduce `Lifecycle`;
- Add initialization and shutdown processes for stateful plugins.

Deliverables:

- Better suited for hosting complex built-in plugins;
- Facilitates evolution to more complete plugin system.

## 11.5 Phase 5: Advanced Capabilities (P2)

Goal: Support more complex plugin ecosystem.

Work items:

- Split queues by event;
- Multi-worker observer runtime;
- Deep tracing / metrics integration;
- Isolation execution strategy for heavy plugins.

---

## 12. Non-Goals

V2 currently **does NOT pursue**:

- Third-party untrusted plugin sandbox;
- Out-of-process RPC plugin system;
- Hot-loading complex plugin ecosystem;
- Complete plugin marketplace.

These capabilities should be evaluated in future higher versions and should not introduce complexity prematurely.

---

## 13. Implementation Recommendations

If only 3 things can be done in the current iteration, the recommended order is:

1. **Do Async Runtime Governance first**
   - This is the step with the highest stability return.

2. **Then do Interceptor / Observer Semantic Separation**
   - This is the most effective step to reduce misuse risk.

3. **Complete Contracts and Meta/Registry**
   - This is the key step to move the system from "usable" to "manageable".

---

## 14. Summary

The goal of V2 is not to package the current Hook system into a "Plugin Platform" that sounds bigger, but to evolve it into a truly:

- Semantically clear;
- Execution controllable;
- Observable;
- Incrementally extensible;
- Extension framework compatible with the current chat main pipeline.

Therefore, the most important thing for V2 is not to "add more plugins", but to first complete the following three things:

- Clearly distinguish **Interceptor** from **Observer**;
- Complete **async runtime boundaries**;
- Establish **payload contracts and plugin metadata**.

After completing these three steps, the Hook system of the current repository will truly have the foundation for long-term evolution.
