# TokenSiren Architecture

## Overview

TokenSiren is a zero-instrumentation observability tool for streaming LLM inference. It uses eBPF probe handlers to observe request and token-stream timing without modifying application code, aggregates metrics in kernel-resident BPF maps, and exports Prometheus metrics for Grafana dashboards.

The first implementation target is a single inference runtime adapter, with vLLM as the intended high-signal runtime target. The architecture is designed so the runtime-specific probe definitions can be swapped or extended later.

## Problem Statement

Existing observability for LLM serving often focuses on request duration, throughput, and infrastructure metrics, but misses the latency shape that users actually feel during streaming inference.

TokenSiren focuses on four primary metrics:

1. **TTFT (Time To First Token)**
   - `first_token_ts - request_start_ts`
   - Measures the delay before the first visible output reaches the stream.

2. **Inter-token latency**
   - `token_n_ts - token_(n-1)_ts`
   - Measures cadence and smoothness of generation after streaming begins.

3. **Stream duration**
   - `stream_end_ts - request_start_ts`
   - Measures total request lifetime.

4. **Tokens per response**
   - Count of emitted token/chunk events per stream.
   - Provides context for duration and throughput.

These metrics will be exported primarily as Prometheus histograms and counters so Grafana can display P50/P95/P99 views, heatmaps, and trend lines.

## Architectural Principles

TokenSiren is intentionally scoped as a tight systems project rather than a large platform. The main design principles are:

- **Zero instrumentation**: no application code changes required.
- **Low overhead**: aggregate in kernel-space maps rather than exporting every event.
- **Legible architecture**: clear separation between kernel data plane and userspace exporter.
- **Single-runtime first**: support one runtime well before generalizing.
- **Mechanical ownership**: keep the implementation small enough to hold mentally and hotfix without AI.

## High-Level Architecture

```text
LLM inference server
  ├─ request / stream lifecycle functions
  ├─ token or chunk emission functions
  └─ optional transport send path

        ↓

eBPF probe handlers
  ├─ read monotonic timestamps
  ├─ look up active stream state
  ├─ compute latency deltas
  ├─ increment histogram buckets
  └─ optionally emit sampled summaries

        ↓

BPF maps
  ├─ active_streams
  ├─ conn_index (optional in earliest v1)
  ├─ metric_buckets
  └─ control

        ↓

Go userspace daemon
  ├─ loads and attaches eBPF programs
  ├─ reads BPF maps
  ├─ translates compact ids into Prometheus labels
  └─ exposes /metrics

        ↓

Prometheus
        ↓

Grafana
```

## Runtime Strategy

### First target

The intended first high-signal runtime target is **vLLM**, running a small self-hosted model for local testing. The model size is not important to the project signal; the serving runtime and observability architecture are.

### Probe strategy

The architecture supports two broad probe classes:

1. **Semantic probes (preferred)**
   - Uprobes attached to request lifecycle or token/chunk emission functions.
   - Best for measuring request start, first token, inter-token cadence, and stream end.

2. **Transport probes (optional later)**
   - Kprobes or tracepoints attached to send-path functions like `tcp_sendmsg`.
   - Useful for semantic-to-wire delay and transport truth.

Earliest v1 may be semantic-only if that keeps the design simpler and more owned.

## BPF Program Model

The BPF side is not a kernel module. It is a set of sandboxed eBPF programs compiled from restricted C into BPF bytecode and loaded into the kernel by the userspace Go daemon.

A useful mental model:

- **Probe** = event trigger
- **eBPF handler** = tiny event-driven lambda running inside the kernel BPF VM
- **BPF map** = tiny kernel-resident key/value state table

Each probe handler should be short and mechanical:

```text
probe fires
↓
lookup stream state
↓
read timestamp
↓
compute delta or update counters
↓
return
```

## Core BPF Maps

### 1. `active_streams`

Purpose:
- Maintain live per-stream timing and token state while a request is active.

Recommended map type:
- `LRU_HASH`

Reasoning:
- This is the hot-path working set.
- LRU behavior provides safety under cleanup misses or churn.

Canonical key:

```c
struct stream_key {
    __u32 tgid;
    __u32 stream_id_hi;
    __u32 stream_id_lo;
    __u32 reserved;
};
```

Notes:
- The architecture prefers a runtime-derived logical stream/request id if available.
- If the runtime cannot provide one cheaply in v1, the implementation may temporarily fall back to a simpler identity shape like `(tgid, fd)`.
- The split `stream_id_hi/lo` layout keeps the struct simple and alignment-friendly for a 64-bit logical id.

Value:

```c
struct stream_state {
    __u64 start_ns;
    __u64 first_token_ns;
    __u64 last_token_ns;
    __u64 end_ns;
    __u32 token_count;
    __u32 label_id;
    __u32 flags;
    __u32 reserved;
};
```

Field meanings:
- `start_ns`: request or stream start timestamp.
- `first_token_ns`: timestamp of the first token/chunk event.
- `last_token_ns`: timestamp of the most recent token/chunk event.
- `end_ns`: optional end timestamp for summaries.
- `token_count`: number of observed token/chunk events.
- `label_id`: compact userspace-managed label dictionary id.
- `flags`: state bits such as first-token-seen, sampled, errored.

Primary use:
- Compute TTFT, inter-token latency, duration, and token count.

### 2. `conn_index` (optional in earliest v1)

Purpose:
- Correlate different probe identity shapes to the canonical `stream_key`.

Recommended map type:
- `HASH`

Canonical key for a transport-aware version:

```c
struct conn_key {
    __u32 tgid;
    __s32 fd;
    __u32 reserved0;
    __u32 reserved1;
};
```

Value:

```c
struct conn_val {
    struct stream_key skey;
};
```

Reasoning:
- Semantic uprobes and transport kprobes may not observe the same key material.
- This map allows translation from fd or socket-adjacent identities to the logical stream.

Status:
- Can be omitted in the earliest semantic-only v1.
- Becomes important once transport probes are added.

### 3. `metric_buckets`

Purpose:
- Hold histogram bucket counts and counters in-kernel so userspace does not need every raw event.

Recommended map type:
- `PERCPU_HASH` if contention becomes meaningful
- otherwise `HASH` for simplicity in earliest v1

Key:

```c
struct metric_key {
    __u32 label_id;
    __u8 metric_type;
    __u8 bucket;
    __u16 reserved;
};
```

Value:

```c
struct metric_val {
    __u64 count;
};
```

Metric types:

```c
enum metric_type {
    METRIC_REQ_TOTAL = 1,
    METRIC_ERR_TOTAL = 2,
    METRIC_TTFT = 3,
    METRIC_INTERTOKEN = 4,
    METRIC_DURATION = 5,
    METRIC_TOKENS = 6,
};
```

Reasoning:
- `label_id` keeps labels compact in the kernel hot path.
- `metric_type` distinguishes request counters vs histograms.
- `bucket` selects the histogram bucket index.

Histogram design:
- Use log-like bucket boundaries suitable for sub-millisecond through second-scale latencies.
- Initial conceptual bucket family:
  - 5 µs, 10 µs, 25 µs, 50 µs
  - 100 µs, 250 µs, 500 µs
  - 1 ms, 2.5 ms, 5 ms, 10 ms
  - 25 ms, 50 ms, 100 ms, 250 ms, 500 ms, 1 s

Primary use:
- TTFT histogram
- inter-token histogram
- duration histogram
- tokens-per-response histogram
- request total counter
- error total counter

### 4. `control`

Purpose:
- Hold runtime-configurable sampling and threshold behavior without recompiling BPF code.

Recommended map type:
- `ARRAY`

Key:

```c
struct control_key {
    __u32 idx;
};
```

Value:

```c
struct control_val {
    __u32 sample_rate;
    __u32 slow_ttft_us;
    __u32 slow_intertoken_us;
    __u32 flags;
};
```

Meaning:
- `sample_rate`: sampling divisor or rate control.
- `slow_ttft_us`: threshold for slow-request summaries.
- `slow_intertoken_us`: threshold for slow inter-token anomalies.
- `flags`: feature toggles such as anomaly export or transport-path enablement.

Reasoning:
- Even a single global config slot is useful in v1.
- Keeps future policy-driven behavior possible without large scope expansion.

### Optional `ringbuf`

Purpose:
- Export sampled or anomalous stream summaries to userspace.

Use:
- Not required for the earliest meaningful version.
- Best introduced after map-based histogram export is working.

Possible summary schema:

```c
struct stream_summary {
    __u64 start_ns;
    __u64 first_token_ns;
    __u64 end_ns;
    __u32 token_count;
    __u32 label_id;
    __u32 flags;
};
```

## `common.h` Schema Draft

The shared BPF header should define the core structs, flags, enums, and bucket constants used by both handlers and userspace bindings.

Draft:

```c
#ifndef TOKENSIREN_COMMON_H
#define TOKENSIREN_COMMON_H

#include <linux/types.h>

#define TOKENSIREN_BUCKET_COUNT 17

struct stream_key {
    __u32 tgid;
    __u32 stream_id_hi;
    __u32 stream_id_lo;
    __u32 reserved;
};

struct stream_state {
    __u64 start_ns;
    __u64 first_token_ns;
    __u64 last_token_ns;
    __u64 end_ns;
    __u32 token_count;
    __u32 label_id;
    __u32 flags;
    __u32 reserved;
};

struct conn_key {
    __u32 tgid;
    __s32 fd;
    __u32 reserved0;
    __u32 reserved1;
};

struct conn_val {
    struct stream_key skey;
};

struct metric_key {
    __u32 label_id;
    __u8 metric_type;
    __u8 bucket;
    __u16 reserved;
};

struct metric_val {
    __u64 count;
};

struct control_key {
    __u32 idx;
};

struct control_val {
    __u32 sample_rate;
    __u32 slow_ttft_us;
    __u32 slow_intertoken_us;
    __u32 flags;
};

struct stream_summary {
    __u64 start_ns;
    __u64 first_token_ns;
    __u64 end_ns;
    __u32 token_count;
    __u32 label_id;
    __u32 flags;
};

enum metric_type {
    METRIC_REQ_TOTAL = 1,
    METRIC_ERR_TOTAL = 2,
    METRIC_TTFT = 3,
    METRIC_INTERTOKEN = 4,
    METRIC_DURATION = 5,
    METRIC_TOKENS = 6,
};

enum stream_flags {
    STREAM_F_FIRST_TOKEN_SEEN = 1 << 0,
    STREAM_F_SAMPLED = 1 << 1,
    STREAM_F_ERRORED = 1 << 2,
    STREAM_F_SUMMARY_EMITTED = 1 << 3,
};

#endif
```

## Handler Responsibilities

The BPF handlers should remain small and bounded.

### Request-start handler
- Construct canonical `stream_key`.
- Initialize `stream_state`.
- Set `start_ns`.
- Populate `label_id`.
- Optionally initialize correlation entries.

### Token/chunk handler
- Look up `active_streams`.
- Read current monotonic timestamp.
- If first token has not been seen:
  - set `first_token_ns`
  - compute TTFT
  - increment TTFT histogram bucket
- Else:
  - compute inter-token delta
  - increment inter-token histogram bucket
- Increment `token_count`
- Update `last_token_ns`

### Stream-end handler
- Look up `active_streams`.
- Set `end_ns`.
- Compute and bucket duration.
- Bucket tokens-per-response.
- Increment request counter or error counter as appropriate.
- Optionally emit sampled summary.
- Delete active and correlation entries.

### Transport handler (optional later)
- Resolve transport-side identity through `conn_index`.
- Compare semantic timing with transport send timing.
- Record transport-delay metrics or summaries.

## Userspace Responsibilities

The Go userspace daemon is responsible for:

- compiling or loading generated BPF bindings
- attaching the selected probes
- reading map contents
- translating `label_id` into Prometheus label sets
- serving `/metrics`

Userspace also owns:
- runtime-specific symbol discovery
- label dictionary management
- control map updates
- future policy reload behavior

## Repository Layout

```text
tokensiren/
├── cmd/
│   └── tokensiren/
│       └── main.go
├── internal/
│   ├── exporter/
│   │   └── prometheus.go
│   ├── probes/
│   │   └── attach.go
│   ├── runtime/
│   │   └── vllm.go
│   └── metrics/
│       └── buckets.go
├── bpf/
│   ├── tracer.c
│   └── common.h
├── gen/
├── dashboards/
│   └── tokensiren.json
├── examples/
│   └── prometheus.yml
├── Makefile
├── go.mod
└── README.md
```

## Scope Boundaries for v1

To keep TokenSiren tractable and owned, v1 explicitly does **not** include:

- multiple runtime adapters at once
- distributed control plane logic
- full raw per-token event export
- universal LLM protocol parsing
- multi-node correlation
- large remote config systems
- broad trace backends beyond Prometheus/Grafana

The v1 success condition is simple:

> Point TokenSiren at one inference runtime, issue a streaming request, and observe TTFT, inter-token latency, duration, and token-count metrics in Prometheus and Grafana.

## Estimated Code Size

Scoped well, the project should remain in a manageable range:

- `bpf/common.h`: ~80–180 LOC
- `bpf/tracer.c`: ~180–350 LOC
- Go userspace loader/exporter/runtime glue: ~400–700 LOC
- dashboards/config/readme/glue: ~100–250 LOC

Expected total for a credible v1:
- **~800–1500 LOC**

This is a deliberate design constraint so the system remains understandable, ownable, and hotfixable.

## Open Decisions

The main remaining implementation decisions are:

1. Exact runtime-specific probe points for vLLM.
2. Whether earliest v1 uses a logical stream id or a simpler `(tgid, fd)` key shape.
3. Whether `metric_buckets` starts as `HASH` or `PERCPU_HASH`.
4. Whether ring-buffer anomaly summaries are included in v1 or deferred.

These can be resolved during implementation without changing the core architecture.

