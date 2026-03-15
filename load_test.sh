#!/usr/bin/env bash
set -euo pipefail

VLLM_URL="${VLLM_URL:-http://127.0.0.1:8999}"
MODEL="${MODEL:-Qwen/Qwen2.5-0.5B-Instruct}"
USERS="${USERS:-6}"
DURATION_SEC="${DURATION_SEC:-3600}"
MAX_INFLIGHT="${MAX_INFLIGHT:-2}"

PROMPTS=(
  "Summarize the main ideas of eBPF observability in 5 bullets."
  "Explain how Prometheus histograms work and when to use them."
  "Write a short story about a system engineer debugging latency."
  "Give a technical overview of vLLM streaming internals."
  "Draft a concise changelog entry for a metrics pipeline."
  "Compare TTFT vs inter-token latency in LLM serving."
  "Generate a checklist for deploying Prometheus and Grafana."
  "Explain why eBPF is useful for low-overhead telemetry."
)

end_ts=$(( $(date +%s) + DURATION_SEC ))

one_request() {
  local prompt="${PROMPTS[$((RANDOM % ${#PROMPTS[@]}))]}"
  local repeat=$((1 + RANDOM % 4))
  local long_prompt=""
  for _ in $(seq 1 $repeat); do
    long_prompt="$long_prompt $prompt"
  done

  local max_tokens=$((30 + RANDOM % 90))
  local temp
  temp=$(awk -v r="$RANDOM" 'BEGIN{printf "%.2f", (r%90)/100 + 0.1}')

  curl -s "$VLLM_URL/v1/completions" \
    -H "Content-Type: application/json" \
    -d "{\n      \"model\":\"$MODEL\",\n      \"prompt\":\"$long_prompt\",\n      \"max_tokens\":$max_tokens,\n      \"temperature\":$temp,\n      \"stream\":true\n    }" >/dev/null
}

run_user() {
  while [ "$(date +%s)" -lt "$end_ts" ]; do
    while [ "$(jobs -pr | wc -l)" -ge "$MAX_INFLIGHT" ]; do
      sleep 0.2
    done
    if (( RANDOM % 100 < 10 )); then
      one_request &
    else
      one_request
    fi
    sleep $(awk -v r="$RANDOM" 'BEGIN{printf "%.2f", (r%200)/100 + 1.0}')
  done
}

for _ in $(seq 1 "$USERS"); do
  run_user &
done
wait
