# TokenSiren v1 TODO

Goal: Attach to a vLLM runtime, collect streaming metrics with eBPF, export Prometheus metrics, and view them in a Grafana dashboard.

## 0. Define v1 acceptance criteria
- [ ] Write down the target environment: Linux kernel version, distro, CPU arch, and whether root or CAP_BPF is available
- [ ] Decide the single runtime target for v1: vLLM only
- [ ] Confirm v1 success metrics: TTFT, inter token latency, stream duration, tokens per response
- [ ] Set minimal operational requirement: `tokensiren` runs, `/metrics` serves data, Grafana dashboard shows non zero series

## 1. Make the BPF program functional
- [ ] Implement `handle_request_start` in `bpf/tracer.c`
- [ ] Construct `stream_key` from runtime inputs or a fallback identity
- [ ] Initialize `stream_state` with `start_ns` and clear other fields
- [ ] Store in `active_streams`
- [ ] Implement `handle_token_emit` in `bpf/tracer.c`
- [ ] Lookup `active_streams` by key
- [ ] Read current time in ns
- [ ] If first token not seen, set `first_token_ns` and bucket TTFT
- [ ] Else compute delta from `last_token_ns` and bucket inter token latency
- [ ] Increment `token_count` and update `last_token_ns`
- [ ] Implement `handle_request_end` in `bpf/tracer.c`
- [ ] Lookup `active_streams`
- [ ] Set `end_ns`
- [ ] Bucket duration and tokens per response
- [ ] Increment request counter or error counter
- [ ] Delete from `active_streams` and `conn_index` if used
- [ ] Add helper for bucket selection in BPF to match `internal/metrics/buckets.go`
- [ ] Decide map types for `metric_buckets` and `active_streams` in v1

## 2. Compile and package the BPF object
- [x] Add a build step in `Makefile` to compile `bpf/tracer.c` into a BPF object
- [ ] Decide object output path and document it
- [ ] Verify the object loads with `bpftool` or the Go loader once implemented

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
- [ ] Update `internal/runtime/vllm.go` to validate all required fields
- [ ] Add defaults or error messages that point to correct symbol names

## 5. Implement Prometheus exporter
- [x] Implement `ServePrometheus` in `internal/exporter/prometheus.go`
  - [ ] Read `metric_buckets` at a fixed interval
  - [ ] Convert bucket counts into Prometheus histograms and counters
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
- [ ] Validate inputs and print a clear startup summary

## 7. Add local run flow
- [ ] Add a minimal runbook section in `README.md`
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
- [ ] Start vLLM with a small model and enable streaming
- [ ] Run `tokensiren` against the runtime
- [ ] Confirm BPF maps are being updated
- [ ] Confirm `/metrics` exports non zero buckets
- [ ] Confirm Grafana dashboard shows TTFT and inter token charts

## 10. Polishing
- [ ] Add structured logs for attach and export lifecycle
- [ ] Add minimal tests for bucket mapping and config validation
- [ ] Add a short troubleshooting section

## 11. Done criteria
- [ ] `tokensiren` runs on a clean machine with only config changes
- [ ] Grafana dashboard shows live TTFT, inter token, duration, and token count
- [ ] CPU overhead remains low for a single runtime target
