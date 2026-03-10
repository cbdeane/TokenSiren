package main

import (
    "log"

    "tokensiren/internal/exporter"
    "tokensiren/internal/probes"
    "tokensiren/internal/runtime"
)

func main() {
    runtimeCfg := runtime.VLLMConfig{}
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
