package exporter

import "errors"

// Handle abstracts the BPF loader so the exporter can read maps.
type Handle interface {
    Close() error
}

func ServePrometheus(handle Handle) error {
    if handle == nil {
        return errors.New("nil handle")
    }
    // TODO: read BPF maps and expose /metrics.
    return nil
}
