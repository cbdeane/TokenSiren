# TokenSiren

TokenSiren is an in progress eBPF based observability tool for streaming LLM inference. The current codebase focuses on the architectural skeleton and types that enable kernel side aggregation and a Go userspace control plane, with vLLM as the first intended runtime target.

## Status

This repository is not yet feature complete. Several components are stubs and exist to lock in interfaces, map schemas, and attach plans for a first viable path.

## Problems To Be Solved:
Modern LLM inference systems expose limited runtime visibility into
token throughput, latency, and request behavior.

TokenSiren instruments inference runtimes using eBPF uprobes and exports
structured telemetry through OpenTelemetry pipelines for analysis
in Prometheus and Grafana.


## Current Architecture

At a high level the system is split into a kernel data plane and a Go userspace control plane.

```
vLLM runtime
      │
   uprobes
      │
   eBPF program
      │
   BPF maps
      │
Go control plane
      │
 Prometheus / OTLP
      │
   Grafana
```

### Kernel data plane

The eBPF program is defined in `bpf/tracer.c` and `bpf/common.h`. It defines map schemas and placeholder probe handlers for request start, token emit, and request end.

Maps currently defined:
- `active_streams` LRU hash for per stream timing state
- `conn_index` hash for optional transport to stream correlation
- `metric_buckets` hash for counters and histogram buckets
- `control` array for runtime tuning knobs

Handlers:
- `handle_request_start`
- `handle_token_emit`
- `handle_request_end`

These handlers are currently empty and are expected to record timestamps and update `metric_buckets` based on the schemas in `bpf/common.h`.

### Userspace control plane

The Go side wires runtime resolution, probe attachment, and metric export.

Flow today:
1. `cmd/tokensiren/main.go` builds a `runtime.VLLMConfig`
2. `internal/runtime/vllm.go` maps that config into a `probes.AttachSpec`
3. `internal/probes/attach.go` validates the spec and will load the BPF object and attach uprobes in a later iteration
4. `internal/exporter/prometheus.go` will read BPF maps and expose `/metrics` once implemented

The only concrete logic in userspace right now is config validation and shape definition. Probe loading and map export are intentionally stubbed.

### Metrics model

The planned latency histogram buckets live in `internal/metrics/buckets.go` as microsecond boundaries that match the architecture draft.

## Repository Layout

```
cmd/tokensiren/          entrypoint wiring
internal/runtime/        runtime resolution for vLLM
internal/probes/         probe attachment interfaces and handles
internal/exporter/       Prometheus exporter placeholder
internal/metrics/        histogram bucket definitions
bpf/                     eBPF program and shared schema
dashboards/              Grafana dashboard JSON
examples/                Prometheus scrape config
```

## Why this matters

This codebase is an example of how to frame a kernel level telemetry pipeline for LLM serving. It shows:
- kernel side map and schema design for high cardinality request and token timing
- a minimal userspace control plane that composes runtime discovery, probe attachment, and metric export
- strict separation between the kernel data plane and the Go services that expose metrics

## Next engineering steps

Near term work is focused on turning the skeleton into a usable vLLM probe pipeline:
- implement uprobe handlers in `bpf/tracer.c`
- load and attach probes in `internal/probes/attach.go`
- export map data to Prometheus in `internal/exporter/prometheus.go`
- wire concrete vLLM symbols and a BPF object path in `internal/runtime/vllm.go`
