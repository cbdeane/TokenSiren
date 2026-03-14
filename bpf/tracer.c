// SPDX-License-Identifier: GPL-2.0
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

#include "common.h"

struct {
    __uint(type, BPF_MAP_TYPE_LRU_HASH);
    __uint(max_entries, 16384);
    __type(key, struct stream_key);
    __type(value, struct stream_state);
} active_streams SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 8192);
    __type(key, struct conn_key);
    __type(value, struct conn_val);
} conn_index SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65536);
    __type(key, struct metric_key);
    __type(value, struct metric_val);
} metric_buckets SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, struct control_key);
    __type(value, struct control_val);
} control SEC(".maps");

static const __u64 latency_buckets_us[TOKENSIREN_BUCKET_COUNT] = {
    5, 10, 25, 50,
    100, 250, 500,
    1000, 2500, 5000, 10000,
    25000, 50000, 100000, 250000, 500000, 1000000,
};

static __always_inline __u8 bucket_for_us(__u64 us)
{
#pragma unroll
    for (int i = 0; i < TOKENSIREN_BUCKET_COUNT; i++) {
        if (us <= latency_buckets_us[i]) {
            return (__u8)i;
        }
    }
    return (TOKENSIREN_BUCKET_COUNT - 1);
}

static __always_inline void inc_metric(__u32 label_id, __u8 metric_type, __u8 bucket)
{
    struct metric_key key = {
        .label_id = label_id,
        .metric_type = metric_type,
        .bucket = bucket,
    };
    struct metric_val *val = bpf_map_lookup_elem(&metric_buckets, &key);
    if (val) {
        __sync_fetch_and_add(&val->count, 1);
        return;
    }
    struct metric_val init = { .count = 1 };
    bpf_map_update_elem(&metric_buckets, &key, &init, BPF_ANY);
}

static __always_inline struct stream_key make_stream_key(void)
{
    __u64 pid_tgid = bpf_get_current_pid_tgid();
    struct stream_key key = {};
    key.tgid = (__u32)(pid_tgid >> 32);
    key.stream_id_lo = 0;
    key.stream_id_hi = 0;
    return key;
}

SEC("uprobe/request_start")
int handle_request_start(struct pt_regs *ctx)
{
    // bpf_printk("tokensiren: request_start hit\n");
    struct stream_key key = make_stream_key();
    struct stream_state state = {};
    state.start_ns = bpf_ktime_get_ns();
    state.last_token_ns = state.start_ns;
    state.label_id = 0;
    state.flags = STREAM_F_SAMPLED;
    bpf_map_update_elem(&active_streams, &key, &state, BPF_ANY);
    inc_metric(state.label_id, METRIC_REQ_TOTAL, 0);
    return 0;
}

SEC("uprobe/token_emit")
int handle_token_emit(struct pt_regs *ctx)
{
    struct stream_key key = make_stream_key();
    struct stream_state *state = bpf_map_lookup_elem(&active_streams, &key);
    if (!state) {
        return 0;
    }
    __u64 now = bpf_ktime_get_ns();
    if (state->first_token_ns == 0) {
        state->first_token_ns = now;
        __u64 ttft_us = (now - state->start_ns) / 1000;
        inc_metric(state->label_id, METRIC_TTFT, bucket_for_us(ttft_us));
    } else {
        __u64 delta_us = (now - state->last_token_ns) / 1000;
        inc_metric(state->label_id, METRIC_INTERTOKEN, bucket_for_us(delta_us));
    }
    state->token_count += 1;
    state->last_token_ns = now;
    return 0;
}

SEC("uprobe/request_end")
int handle_request_end(struct pt_regs *ctx)
{
    struct stream_key key = make_stream_key();
    struct stream_state *state = bpf_map_lookup_elem(&active_streams, &key);
    if (!state) {
        return 0;
    }
    __u64 now = bpf_ktime_get_ns();
    state->end_ns = now;

    __u64 dur_us = (now - state->start_ns) / 1000;
    inc_metric(state->label_id, METRIC_DURATION, bucket_for_us(dur_us));

    __u32 tokens = state->token_count;
    inc_metric(state->label_id, METRIC_TOKENS, bucket_for_us(tokens));
    inc_metric(state->label_id, METRIC_REQ_TOTAL, 0);

    bpf_map_delete_elem(&active_streams, &key);
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
