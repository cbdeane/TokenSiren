#ifndef TOKENSIREN_COMMON_H
#define TOKENSIREN_COMMON_H

#include <linux/types.h>

#define TOKENSIREN_BUCKET_COUNT 17

struct stream_key {
    __u32 tgid;
    __u32 stream_id_hi;
    __u32 stream_id_lo;
    __u32 reserved;
};

struct stream_state {
    __u64 start_ns;
    __u64 first_token_ns;
    __u64 last_token_ns;
    __u64 end_ns;
    __u32 token_count;
    __u32 label_id;
    __u32 flags;
    __u32 reserved;
};

struct conn_key {
    __u32 tgid;
    __s32 fd;
    __u32 reserved0;
    __u32 reserved1;
};

struct conn_val {
    struct stream_key skey;
};

struct metric_key {
    __u32 label_id;
    __u8 metric_type;
    __u8 bucket;
    __u16 reserved;
};

struct metric_val {
    __u64 count;
};

struct control_key {
    __u32 idx;
};

struct control_val {
    __u32 sample_rate;
    __u32 slow_ttft_us;
    __u32 slow_intertoken_us;
    __u32 flags;
};

struct stream_summary {
    __u64 start_ns;
    __u64 first_token_ns;
    __u64 end_ns;
    __u32 token_count;
    __u32 label_id;
    __u32 flags;
};

enum metric_type {
    METRIC_REQ_TOTAL = 1,
    METRIC_ERR_TOTAL = 2,
    METRIC_TTFT = 3,
    METRIC_INTERTOKEN = 4,
    METRIC_DURATION = 5,
    METRIC_TOKENS = 6,
};

enum stream_flags {
    STREAM_F_FIRST_TOKEN_SEEN = 1 << 0,
    STREAM_F_SAMPLED = 1 << 1,
    STREAM_F_ERRORED = 1 << 2,
    STREAM_F_SUMMARY_EMITTED = 1 << 3,
};

#endif
