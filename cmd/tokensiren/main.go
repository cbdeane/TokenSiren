package main

import (
    "log"
    "os"
    "strings"

    "tokensiren/internal/exporter"
    "tokensiren/internal/probes"
    "tokensiren/internal/runtime"
)

func main() {
    if err := loadDotEnv(".env"); err != nil {
        log.Fatalf("load .env: %v", err)
    }

    runtimeCfg := runtime.VLLMConfig{
        BinaryPath:   os.Getenv("TOKENSIREN_BINARY_PATH"),
        BPFObject:    os.Getenv("TOKENSIREN_BPF_OBJECT"),
        RequestStart: os.Getenv("TOKENSIREN_REQUEST_START"),
        TokenEmit:    os.Getenv("TOKENSIREN_TOKEN_EMIT"),
        RequestEnd:   os.Getenv("TOKENSIREN_REQUEST_END"),
    }
    attachSpec, err := runtime.ResolveVLLM(runtimeCfg)
    if err != nil {
        log.Fatalf("resolve runtime: %v", err)
    }

    handle, err := probes.Attach(attachSpec)
    if err != nil {
        log.Fatalf("attach probes: %v", err)
    }
    defer handle.Close()

    if err := exporter.ServePrometheus(handle); err != nil {
        log.Fatalf("serve metrics: %v", err)
    }
}

func loadDotEnv(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            return nil
        }
        return err
    }

    lines := strings.Split(string(data), "\n")
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        if strings.HasPrefix(line, "export ") {
            line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
        }
        eq := strings.Index(line, "=")
        if eq <= 0 {
            continue
        }
        key := strings.TrimSpace(line[:eq])
        val := strings.TrimSpace(line[eq+1:])
        if len(val) >= 2 {
            if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
                val = val[1 : len(val)-1]
            }
        }
        if _, exists := os.LookupEnv(key); !exists {
            _ = os.Setenv(key, val)
        }
    }
    return nil
}
