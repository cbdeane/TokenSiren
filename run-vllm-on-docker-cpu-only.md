docker run --rm -it \
  -p 8000:8000 \
  --security-opt seccomp=unconfined \
  --cap-add SYS_NICE \
  --shm-size=4g \
  -v ~/.cache/huggingface:/root/.cache/huggingface \
  -e HF_TOKEN="$HF_TOKEN" \
  -e VLLM_CPU_KVCACHE_SPACE=4 \
  vllm/vllm-openai-cpu:latest \
  Qwen/Qwen3-0.6B \
  --dtype float \
  --max-model-len 4096 \
  --host 0.0.0.0 \
  --port 8000
