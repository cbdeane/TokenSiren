package runtime

import (
    "fmt"

    "tokensiren/internal/probes"
)

type VLLMConfig struct {
    BinaryPath   string
    BPFObject    string
    RequestStart string
    TokenEmit    string
    RequestEnd   string
}

func ResolveVLLM(cfg VLLMConfig) (probes.AttachSpec, error) {
    if cfg.BPFObject == "" {
        return probes.AttachSpec{}, fmt.Errorf("missing BPFObject (set TOKENSIREN_BPF_OBJECT, e.g. gen/tracer.o)")
    }
    if cfg.BinaryPath == "" {
        return probes.AttachSpec{}, fmt.Errorf("missing BinaryPath (set TOKENSIREN_BINARY_PATH; example: /proc/<pid>/root/opt/venv/lib/python3.12/site-packages/vllm/_C.abi3.so)")
    }
    if cfg.RequestStart == "" {
        return probes.AttachSpec{}, fmt.Errorf("missing RequestStart (set TOKENSIREN_REQUEST_START to an ELF symbol; see symbol-table-lookup.md for examples)")
    }
    if cfg.TokenEmit == "" {
        return probes.AttachSpec{}, fmt.Errorf("missing TokenEmit (set TOKENSIREN_TOKEN_EMIT to an ELF symbol; see symbol-table-lookup.md for examples)")
    }
    if cfg.RequestEnd == "" {
        return probes.AttachSpec{}, fmt.Errorf("missing RequestEnd (set TOKENSIREN_REQUEST_END to an ELF symbol; see symbol-table-lookup.md for examples)")
    }
    return probes.AttachSpec{
        BPFObjectPath: cfg.BPFObject,
        Uprobes: []probes.UprobeSpec{
            {
                BinaryPath: cfg.BinaryPath,
                Symbol:     cfg.RequestStart,
                Handler:    "handle_request_start",
            },
            {
                BinaryPath: cfg.BinaryPath,
                Symbol:     cfg.TokenEmit,
                Handler:    "handle_token_emit",
            },
            {
                BinaryPath: cfg.BinaryPath,
                Symbol:     cfg.RequestEnd,
                Handler:    "handle_request_end",
            },
        },
    }, nil
}
