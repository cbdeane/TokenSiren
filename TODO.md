# TokenSiren v1 TODO

Goal: Attach to a vLLM runtime, collect streaming metrics with eBPF, export Prometheus metrics, and view them in a Grafana dashboard.

## 0. Define v1 acceptance criteria
- [x] Write down the target environment: Linux x86_64, run with sudo for BPF access
- [x] Decide the single runtime target for v1: vLLM only
- [x] Confirm v1 success metrics for Grafana (use current probe coverage; no additional probe hunting):
  - Active Streams
  - Tokens/s
  - Requests/s
  - P50/P95/P99 request latency
  - Token interarrival latency
  - Tokens per request distribution
  - TTFT (if feasible in v1)
- [ ] Set minimal operational requirement: Grafana dashboard shows non-zero series for confirmed metrics (include screenshot in repo)

## 1. Make the BPF program functional
- [x] Implement `handle_request_start` in `bpf/tracer.c`
- [x] Construct `stream_key` from runtime inputs or a fallback identity
- [x] Initialize `stream_state` with `start_ns` and clear other fields
- [x] Store in `active_streams`
- [x] Implement `handle_token_emit` in `bpf/tracer.c`
- [x] Lookup `active_streams` by key
- [x] Read current time in ns
- [x] If first token not seen, set `first_token_ns` and bucket TTFT
- [x] Else compute delta from `last_token_ns` and bucket inter token latency
- [x] Increment `token_count` and update `last_token_ns`
- [x] Implement `handle_request_end` in `bpf/tracer.c`
- [x] Lookup `active_streams`
- [x] Set `end_ns`
- [x] Bucket duration and tokens per response
- [x] Increment request counter or error counter
- [x] Delete from `active_streams` and `conn_index` if used
- [x] Add helper for bucket selection in BPF to match `internal/metrics/buckets.go`
- [x] Decide map types for `metric_buckets` and `active_streams` in v1
  - [x] Find a per-token/chunk emission symbol so `handle_token_emit` fires per token
  - [x] Find a true request/stream end symbol so duration/tokens-per-request are correct

## 2. Compile and package the BPF object
- [x] Add a build step in `Makefile` to compile `bpf/tracer.c` into a BPF object
- [x] Decide object output path and document it (`gen/tracer.o`)
- [x] Verify the object loads with `bpftool` or the Go loader once implemented

## 3. Implement probe attachment in Go
- [x] Implement `Attach` in `internal/probes/attach.go`
- [x] Load the BPF object with libbpf or cilium ebpf
- [x] Open BPF maps and keep handles in `Handle`
- [x] Attach uprobes for request start, token emit, request end
- [x] Provide `Close` that detaches probes and closes maps
- [x] Add errors with clear context for missing symbols or binaries

## 4. Implement vLLM runtime resolution
- [x] Decide how to discover vLLM binary and symbols
- [x] Fixed config file, env vars, or CLI flags
- [x] Map function names to `RequestStart`, `TokenEmit`, `RequestEnd`
- [x] Update `internal/runtime/vllm.go` to validate all required fields
- [x] Add defaults or error messages that point to correct symbol names

## 5. Implement Prometheus exporter
- [x] Implement `ServePrometheus` in `internal/exporter/prometheus.go`
  - [x] Read `metric_buckets` at a fixed interval
  - [x] Convert bucket counts into Prometheus histograms and counters
  - [x] Expose `/metrics` on a configurable port
- [ ] Choose label strategy
- [ ] Start with a minimal set: runtime, model, status
- [ ] Keep label cardinality bounded

## 6. Wire CLI or config
- [x] Add CLI flags or config file parsing in `cmd/tokensiren/main.go`
- [x] BPF object path
- [x] vLLM binary path
- [x] symbol names
- [x] metrics listen address
- [x] Validate inputs and print a clear startup summary

## 7. Add local run flow
- [x] Add a minimal runbook section in `README.md`
- [ ] Build steps
- [ ] Example config
- [ ] Required kernel features
- [ ] How to run Prometheus and Grafana
- [ ] Update `examples/prometheus.yml` if the metrics endpoint changes

## 8. Prepare for Kubernetes DaemonSet + ConfigMap
- [x] Define env var contract for config (ConfigMap + `.env`)
- [ ] Add a DaemonSet manifest template
- [ ] `hostPID: true`
- [ ] `privileged: true` or explicit caps (`BPF`, `SYS_ADMIN`, `PERFMON`, `SYS_RESOURCE`)
- [ ] HostPath mounts for `/proc`, `/sys/fs/bpf`, `/sys/kernel/debug`
- [ ] Prometheus scrape port + Service (or hostPort) definition
- [ ] Document target process discovery strategy
- [ ] Resolve container PID via `/proc` and cgroups
- [ ] Map to binary path via `/proc/<pid>/root/...`
- [ ] Add example ConfigMap with symbol names and binary path template
- [ ] Note runtime-specific considerations (containerd vs Docker path resolution)

## 9. End to end validation
- [x] Start vLLM with a small model and enable streaming
- [x] Run `tokensiren` against the runtime
- [x] Confirm BPF maps are being updated
- [x] Confirm `/metrics` exports non zero buckets
- [ ] Confirm Grafana dashboard shows TTFT and inter token charts

## 10. Polishing
- [ ] Add structured logs for attach and export lifecycle
- [ ] Add minimal tests for bucket mapping and config validation
- [ ] Add a short troubleshooting section

## 11. Done criteria
- [ ] `tokensiren` runs on a clean machine with only config changes
- [ ] Grafana dashboard shows live TTFT, inter token, duration, and token count
- [ ] CPU overhead remains low for a single runtime target

## 12. Follow-up cleanup
- [ ] Revisit `.env.example` once the Kubernetes/DaemonSet flow is finalized
