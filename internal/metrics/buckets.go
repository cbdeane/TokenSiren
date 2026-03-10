package metrics

// LatencyBucketsUS matches the conceptual bucket plan in the architecture doc.
var LatencyBucketsUS = []uint64{
    5, 10, 25, 50,
    100, 250, 500,
    1000, 2500, 5000, 10000,
    25000, 50000, 100000, 250000, 500000, 1000000,
}
