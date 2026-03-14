package exporter

import (
    "errors"
    "net/http"
    "os"
)

// Handle abstracts the BPF loader so the exporter can read maps.
type Handle interface {
    Close() error
}

func ServePrometheus(handle Handle) error {
    if handle == nil {
        return errors.New("nil handle")
    }
    addr := os.Getenv("TOKENSIREN_METRICS_ADDR")
    if addr == "" {
        addr = ":2112"
    }

    mux := http.NewServeMux()
    mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
        w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
        _, _ = w.Write([]byte("# TokenSiren metrics are not implemented yet\n"))
    })

    srv := &http.Server{
        Addr:    addr,
        Handler: mux,
    }

    return srv.ListenAndServe()
}
