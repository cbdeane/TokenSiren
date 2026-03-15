# Symbol table lookup notes (vLLM CPU container)

This document captures the exact process I used to locate probeable symbols for vLLM in a running Docker container and the concrete symbols I found.

## Environment

- Container image: `vllm/vllm-openai-cpu:latest`
- Container name: `pedantic_blackwell`
- vLLM process list (inside container):
  - PID 1: `/opt/venv/bin/python3 /opt/venv/bin/vllm serve ...`
  - PID 111: `VLLM::EngineCore`

## Goal

Find stable, probeable symbols in a binary loaded by the `VLLM::EngineCore` process so I can attach uprobes from TokenSiren.

## Process

1) **Identify the EngineCore process**

I verified that the process list includes a separate `VLLM::EngineCore` process (PID 111). This indicates that the hot path is not the Python entrypoint.

2) **Verify the EngineCore executable**

`nm -D` against `/proc/111/exe` only returned Python runtime symbols, confirming the actual hot-path functions are in shared libraries loaded by the process.

3) **List mapped shared libraries**

I enumerated libraries mapped by PID 111 via `/proc/111/maps` to find the relevant `.so` files.

4) **Check vLLM extension modules**

I located the vLLM package directory and its C++ extensions:

- `/opt/venv/lib/python3.12/site-packages/vllm/_C.abi3.so`
- `/opt/venv/lib/python3.12/site-packages/vllm/_C_AVX2.abi3.so`

5) **Find candidate symbols**

`strings -a` on `_C.abi3.so` showed function names like `mla_decode_kvcache` and `per_token_quant_int8_cpu`, which suggested relevant compute kernels.

6) **Confirm symbols are probeable**

I used `nm -a --demangle` and filtered for those names. This confirmed the symbols are present in the full symbol table (not just in strings).

7) **Confirm which extension is actually loaded**

I checked `/proc/111/maps` to confirm the process is loading `_C.abi3.so` (not `_C_AVX2.abi3.so`).

8) **Resolve mangled names for candidate symbols**

Get mangled + demangled names in one pass:

```
nm -D --defined-only /opt/venv/lib/python3.12/site-packages/vllm/_C.abi3.so \
  | awk '{print $1, $2, $3}' \
  | while read a t s; do echo "$a $t $s | $(c++filt $s)"; done \
  | grep -E 'cpu_attention_with_kv_cache|cpu_attn_reshape_and_cache'
```

9) **Verify symbol hits with bpftrace**

Run on the host (use the host PID for `VLLM::EngineCore`):

```
sudo bpftrace -e 'uprobe:/proc/<HOST_PID>/root/opt/venv/lib/python3.12/site-packages/vllm/_C.abi3.so:_Z26cpu_attn_reshape_and_cacheRKN2at6TensorES2_RS0_S3_S2_RKNSt7__cxx1112basic_stringIcSt11char_traitsIcESaIcEEE { @[pid] = count(); }'
```

```
sudo bpftrace -e 'uprobe:/proc/<HOST_PID>/root/opt/venv/lib/python3.12/site-packages/vllm/_C.abi3.so:_Z27cpu_attention_with_kv_cacheRKN2at6TensorES2_S2_RS0_S2_S2_dbRKSt8optionalIS0_EllS2_dS2_S7_ { @[pid] = count(); }'
```

Send a vLLM request while each runs, then Ctrl-C to see counts. Non-zero counts confirm the symbol fires in the current CPU path.

## Concrete results

Loaded binary (from `/proc/111/maps`):

- `/opt/venv/lib/python3.12/site-packages/vllm/_C.abi3.so`

Probeable symbols found (from `nm -a --demangle`):

- `mla_decode_kvcache(at::Tensor&, at::Tensor&, at::Tensor&, double, at::Tensor&, at::Tensor&)`
- `per_token_quant_int8_cpu(at::Tensor&)`
- `cpu_attn_reshape_and_cache(at::Tensor const&, at::Tensor const&, at::Tensor&, at::Tensor&, at::Tensor const&, std::string const&)`
- `cpu_attention_with_kv_cache(at::Tensor const&, at::Tensor const&, at::Tensor const&, at::Tensor&, at::Tensor const&, at::Tensor const&, double, bool, std::optional<at::Tensor> const&, long, long, at::Tensor const&, double, at::Tensor const&, std::optional<at::Tensor> const&)`

These are compute-level functions (not explicit request start/end markers), but they are valid uprobe targets and sufficient to wire the initial attach path.

### Mangled symbol names (required for `link.OpenExecutable().Uprobe`)

The Go uprobe attach uses ELF symbol names, so the mangled symbols are required. From `nm -a` (non-demangled):

- `_Z18mla_decode_kvcacheRN2at6TensorES1_S1_dS1_S1_`
- `_Z24per_token_quant_int8_cpuRN2at6TensorE`
- `_Z26cpu_attn_reshape_and_cacheRKN2at6TensorES2_RS0_S3_S2_RKNSt7__cxx1112basic_stringIcSt11char_traitsIcESaIcEEE`
- `_Z27cpu_attention_with_kv_cacheRKN2at6TensorES2_S2_RS0_S2_S2_dbRKSt8optionalIS0_EllS2_dS2_S7_`

### Probe hit verification

The original `_Z18mla_decode_kvcache...` and `_Z24per_token_quant_int8_cpu...` symbols attached but did not fire under live load.

The following mangled symbols **did** fire under `bpftrace` with the EngineCore PID:

- `_Z26cpu_attn_reshape_and_cacheRKN2at6TensorES2_RS0_S3_S2_RKNSt7__cxx1112basic_stringIcSt11char_traitsIcESaIcEEE`
- `_Z27cpu_attention_with_kv_cacheRKN2at6TensorES2_S2_RS0_S2_S2_dbRKSt8optionalIS0_EllS2_dS2_S7_`

## Suggested initial mapping for TokenSiren

- `BinaryPath`: `/opt/venv/lib/python3.12/site-packages/vllm/_C.abi3.so`
- `RequestStart`: `_Z26cpu_attn_reshape_and_cacheRKN2at6TensorES2_RS0_S3_S2_RKNSt7__cxx1112basic_stringIcSt11char_traitsIcESaIcEEE`
- `TokenEmit`: `_Z27cpu_attention_with_kv_cacheRKN2at6TensorES2_S2_RS0_S2_S2_dbRKSt8optionalIS0_EllS2_dS2_S7_`
- `RequestEnd`: `_Z27cpu_attention_with_kv_cacheRKN2at6TensorES2_S2_RS0_S2_S2_dbRKSt8optionalIS0_EllS2_dS2_S7_`
- `BPFObject`: `gen/tracer.o` (host build output)

## Notes / caveats

- These symbols are compute kernels; higher-level request boundaries likely require different hooks (e.g., scheduler or request lifecycle code in vLLM Python/C++ layers).
- If symbols change across vLLM versions, repeat the same process: map loaded libs, scan with `strings`, then verify with `nm -a`.

## Upstream patch: stream_end_hook (request completion)

I added a small upstream patch that exposes a stable, probeable request-completion hook in vLLM. The patch is saved in this repo at:

- `./upstream/stream_end_hook.patch`

### What the patch does

- Adds no-op C++ symbols `stream_start_hook()`, `stream_emit_hook()`, and `stream_end_hook()` exported from the vLLM CPU extension.
- Registers custom ops `stream_start_hook(Tensor) -> ()`, `stream_emit_hook(Tensor) -> ()`, and `stream_end_hook(Tensor) -> ()`.
- Calls `torch.ops._C.stream_start_hook(torch.empty(0))` at the beginning of streaming generators.
- Calls `torch.ops._C.stream_emit_hook(torch.empty(0))` immediately before emitting each streaming SSE chunk.
- Calls `torch.ops._C.stream_end_hook(torch.empty(0))` right before emitting the terminal `[DONE]` event.

This provides a stable uprobe target at the exact end of a streaming request.

### Probe validation notes

The `stream_end_hook` symbol is introduced by this patch (it does not exist in stock vLLM). The upstream diff is here:

- `./upstream/stream_end_hook.patch`

On CPU builds, the symbols are present in the vLLM extension module:

- `/home/char0/dev/vllm/vllm/_C.abi3.so`
- `nm -D --defined-only` shows `stream_start_hook`, `stream_emit_hook`, and `stream_end_hook`.

### Preferred vLLM mapping for TokenSiren (patched build)

For the patched vLLM build, use the stream hook symbols directly so all events are emitted in the API server process:

- `RequestStart`: `stream_start_hook`
- `TokenEmit`: `stream_emit_hook`
- `RequestEnd`: `stream_end_hook`

This avoids cross-process keying issues between EngineCore and the API server and produces correct TTFT/intertoken/duration/tokens metrics.

Uprobe example:

```
sudo bpftrace -e 'uprobe:/home/char0/dev/vllm/vllm/_C.abi3.so:stream_end_hook { printf("stream_end_hook\n"); }'
```

If the symbol does not fire, verify that the process running the streaming generator has `_C.abi3.so` mapped in `/proc/<PID>/maps`.
