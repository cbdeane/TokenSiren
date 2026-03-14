package exporter

import (
    "errors"
    "fmt"
    "net/http"
    "os"
    "strconv"
    "strings"

    "github.com/cilium/ebpf"
    "tokensiren/internal/metrics"
)

// Handle abstracts the BPF loader so the exporter can read maps.
type Handle interface {
    Close() error
    MetricBuckets() *ebpf.Map
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
        out, err := renderMetrics(handle.MetricBuckets())
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
            return
        }
        _, _ = w.Write([]byte(out))
    })

    srv := &http.Server{
        Addr:    addr,
        Handler: mux,
    }

    return srv.ListenAndServe()
}

type metricKey struct {
    LabelID    uint32
    MetricType uint8
    Bucket     uint8
    Reserved   uint16
}

type metricVal struct {
    Count uint64
}

func renderMetrics(m *ebpf.Map) (string, error) {
    if m == nil {
        return "", errors.New("metric_buckets map is nil")
    }

    var b strings.Builder
    b.WriteString("# TYPE tokensiren_metric_bucket counter\n")

    it := m.Iterate()
    var key metricKey
    var val metricVal
    for it.Next(&key, &val) {
        metricName := metricTypeName(key.MetricType)
        bucket := bucketLabel(key.Bucket)
        b.WriteString("tokensiren_metric_bucket{metric_type=\"")
        b.WriteString(metricName)
        b.WriteString("\",label_id=\"")
        b.WriteString(strconv.FormatUint(uint64(key.LabelID), 10))
        b.WriteString("\",bucket=\"")
        b.WriteString(bucket)
        b.WriteString("\"} ")
        b.WriteString(strconv.FormatUint(val.Count, 10))
        b.WriteString("\n")
    }
    if err := it.Err(); err != nil {
        return "", fmt.Errorf("iterate metric_buckets: %w", err)
    }

    return b.String(), nil
}

func metricTypeName(t uint8) string {
    switch t {
    case 1:
        return "req_total"
    case 2:
        return "err_total"
    case 3:
        return "ttft"
    case 4:
        return "intertoken"
    case 5:
        return "duration"
    case 6:
        return "tokens"
    default:
        return "unknown"
    }
}

func bucketLabel(idx uint8) string {
    if int(idx) < len(metrics.LatencyBucketsUS) {
        return strconv.FormatUint(metrics.LatencyBucketsUS[idx], 10)
    }
    return "inf"
}
