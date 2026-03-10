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

SEC("uprobe/request_start")
int handle_request_start(struct pt_regs *ctx)
{
    return 0;
}

SEC("uprobe/token_emit")
int handle_token_emit(struct pt_regs *ctx)
{
    return 0;
}

SEC("uprobe/request_end")
int handle_request_end(struct pt_regs *ctx)
{
    return 0;
}

char LICENSE[] SEC("license") = "GPL";
