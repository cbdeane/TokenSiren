package exporter

import (
    "errors"
    "fmt"
    "net/http"
    "os"
    "strconv"
    "strings"
    "sync"
    "time"

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

    cache := &metricsCache{}
    snapshotInterval := 1 * time.Second
    refreshSnapshot(cache, handle.MetricBuckets())
    go func() {
        ticker := time.NewTicker(snapshotInterval)
        defer ticker.Stop()
        for range ticker.C {
            refreshSnapshot(cache, handle.MetricBuckets())
        }
    }()

    mux := http.NewServeMux()
    mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
        w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
        out, err := cache.snapshot()
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

type metricsCache struct {
    mu   sync.RWMutex
    text string
    err  error
}

func (c *metricsCache) snapshot() (string, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.err != nil {
        return "", c.err
    }
    return c.text, nil
}

func refreshSnapshot(c *metricsCache, m *ebpf.Map) {
    text, err := renderMetrics(m)
    c.mu.Lock()
    c.text = text
    c.err = err
    c.mu.Unlock()
}

func renderMetrics(m *ebpf.Map) (string, error) {
    if m == nil {
        return "", errors.New("metric_buckets map is nil")
    }

    it := m.Iterate()
    var (
        key metricKey
        val metricVal
    )
    data := make(map[uint8]map[uint32]map[uint8]uint64)
    for it.Next(&key, &val) {
        byType := data[key.MetricType]
        if byType == nil {
            byType = make(map[uint32]map[uint8]uint64)
            data[key.MetricType] = byType
        }
        byLabel := byType[key.LabelID]
        if byLabel == nil {
            byLabel = make(map[uint8]uint64)
            byType[key.LabelID] = byLabel
        }
        byLabel[key.Bucket] += val.Count
    }
    if err := it.Err(); err != nil {
        return "", fmt.Errorf("iterate metric_buckets: %w", err)
    }

    var b strings.Builder

    renderCounter(&b, "tokensiren_requests_total", data[1])
    renderCounter(&b, "tokensiren_errors_total", data[2])

    renderHistogram(&b, "tokensiren_ttft_us", data[3], metrics.LatencyBucketsUS)
    renderHistogram(&b, "tokensiren_intertoken_us", data[4], metrics.LatencyBucketsUS)
    renderHistogram(&b, "tokensiren_duration_us", data[5], metrics.LatencyBucketsUS)
    renderHistogram(&b, "tokensiren_tokens", data[6], metrics.LatencyBucketsUS)

    return b.String(), nil
}

func renderCounter(b *strings.Builder, name string, data map[uint32]map[uint8]uint64) {
    if data == nil {
        return
    }
    b.WriteString("# TYPE ")
    b.WriteString(name)
    b.WriteString(" counter\n")
    for labelID, buckets := range data {
        var total uint64
        for _, v := range buckets {
            total += v
        }
        b.WriteString(name)
        b.WriteString("{label_id=\"")
        b.WriteString(strconv.FormatUint(uint64(labelID), 10))
        b.WriteString("\"} ")
        b.WriteString(strconv.FormatUint(total, 10))
        b.WriteString("\n")
    }
}

func renderHistogram(b *strings.Builder, name string, data map[uint32]map[uint8]uint64, bounds []uint64) {
    if data == nil {
        return
    }
    b.WriteString("# TYPE ")
    b.WriteString(name)
    b.WriteString(" histogram\n")

    for labelID, buckets := range data {
        var cumulative uint64
        var sum float64
        for i, bound := range bounds {
            count := buckets[uint8(i)]
            cumulative += count
            sum += float64(bound) * float64(count)
            b.WriteString(name)
            b.WriteString("_bucket{label_id=\"")
            b.WriteString(strconv.FormatUint(uint64(labelID), 10))
            b.WriteString("\",le=\"")
            b.WriteString(strconv.FormatUint(bound, 10))
            b.WriteString("\"} ")
            b.WriteString(strconv.FormatUint(cumulative, 10))
            b.WriteString("\n")
        }
        b.WriteString(name)
        b.WriteString("_bucket{label_id=\"")
        b.WriteString(strconv.FormatUint(uint64(labelID), 10))
        b.WriteString("\",le=\"+Inf\"} ")
        b.WriteString(strconv.FormatUint(cumulative, 10))
        b.WriteString("\n")

        b.WriteString(name)
        b.WriteString("_count{label_id=\"")
        b.WriteString(strconv.FormatUint(uint64(labelID), 10))
        b.WriteString("\"} ")
        b.WriteString(strconv.FormatUint(cumulative, 10))
        b.WriteString("\n")

        b.WriteString(name)
        b.WriteString("_sum{label_id=\"")
        b.WriteString(strconv.FormatUint(uint64(labelID), 10))
        b.WriteString("\"} ")
        b.WriteString(strconv.FormatFloat(sum, 'f', -1, 64))
        b.WriteString("\n")
    }
}
