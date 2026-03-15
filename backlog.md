# Backlog

## Future work

- Find a true request/stream completion symbol for vLLM CPU and restore a distinct
  `RequestEnd` handler. At that point:
  - compute duration in `handle_request_end`
  - compute tokens-per-request in `handle_request_end`
  - delete the stream state there
- Find a per-token/chunk emission symbol in the CPU path so `handle_token_emit`
  triggers for each emitted token.
- Revisit token bucket boundaries (tokens are currently bucketed with latency
  boundaries, which is suboptimal).
