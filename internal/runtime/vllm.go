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
        return probes.AttachSpec{}, fmt.Errorf("missing BPFObject")
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
