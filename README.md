# inferctl

One manifest from local Ollama to Kubernetes vLLM.

The local-to-cloud AI deployment bridge that doesn't exist yet.

## The problem

165K developers use Ollama locally. 73K use vLLM in production. There is no tool that keeps a single config consistent across both. Teams manually rewrite configs, Helm charts, and deployment scripts every time they promote a model from laptop to cluster.

## How it works

```
                    model.yaml
                        │
            ┌───────────┼───────────┐
            ▼                       ▼
     inferctl dev              inferctl gen
            │                       │
        ┌───▼───┐         ┌────────▼────────┐
        │ Ollama │         │ K8s manifests   │
        │ local  │         │ - vLLM Deploy   │
        └───────┘         │ - HPA            │
                          │ - HTTPRoute      │
                          │ - InferenceModel │
                          │ - NetworkPolicy  │
                          └────────┬────────┘
                                   │
                            inferctl apply
                                   │
                              ┌────▼────┐
                              │ kubectl │
                              │ apply   │
                              └─────────┘
```

## Quick start

```bash
go install github.com/amayabdaniel/inferctl@latest
```

### 1. Define your model

```yaml
# model.yaml
name: support-chat
model: qwen3:8b
context_length: 8192
quantization: q4_k_m
prompt_template: |
  You are a helpful customer support agent.
observability:
  metrics: true
  tracing: true
scaling:
  min_replicas: 1
  max_replicas: 4
  target_tokens_per_second: 500
resources:
  gpu: nvidia-l4
  gpu_count: 1
  memory_mi: 16384
security:
  prompt_injection_protection: true
  pii_redaction: true
```

### 2. Check GPU compatibility

```
$ inferctl info -f model.yaml

Model: support-chat
  Ollama:      qwen3:8b
  HuggingFace: Qwen/Qwen3-8B
  Parameters:  8B
  Est. VRAM:   5.0 GB
  Quantization: q4_k_m
  Context:     8192 tokens

GPU Compatibility:
  T4               16 GB  $0.35/hr  [BEST]
  L4               24 GB  $0.80/hr  [OK]
  A10G             24 GB  $1.01/hr  [OK]
  A100-40GB        40 GB  $3.40/hr  [OK]
  A100-80GB        80 GB  $4.10/hr  [OK]
  H100             80 GB  $8.00/hr  [OK]

Scaling: 1-4 replicas, target 500 tokens/sec
```

### 3. Run locally

```bash
inferctl dev -f model.yaml
```

### 4. Generate K8s manifests

```bash
inferctl gen -f model.yaml -o ./k8s/
```

Generates:
- `support-chat-vllm.yaml` — Deployment + Service + HPA with GPU scheduling
- `support-chat-gateway.yaml` — Gateway API HTTPRoute + InferenceModel + NetworkPolicy

### 5. Deploy to cluster

```bash
inferctl apply -f model.yaml -n inference --context prod

# Or preview first
inferctl apply -f model.yaml --dry-run
```

## Model name mapping

inferctl automatically converts between Ollama and HuggingFace names:

| model.yaml | Ollama (local) | vLLM (K8s) |
|---|---|---|
| `qwen3:8b` | `qwen3:8b` | `Qwen/Qwen3-8B` |
| `llama3.3:70b` | `llama3.3:70b` | `meta-llama/Llama-3.3-70B-Instruct` |
| `deepseek-r1:7b` | `deepseek-r1:7b` | `deepseek-ai/DeepSeek-R1-Distill-Qwen-7B` |
| `ministral:8b` | `ministral:8b` | `mistralai/Ministral-8B-Instruct-2410` |

15 models in the built-in registry. Unknown models pass through as-is.

## Security

Generated manifests include:
- `runAsNonRoot: true`
- `seccompProfile: RuntimeDefault`
- `capabilities: drop: ["ALL"]`
- `readOnlyRootFilesystem` where possible
- NetworkPolicy isolating inference pods (gateway-only ingress)

Input validation:
- DNS-compatible names only
- Shell injection prevention
- Path traversal detection
- Quantization allowlist
- Tool endpoint scheme validation

## What gets generated

From a single `model.yaml`, inferctl produces:

| Resource | Purpose |
|---|---|
| `Deployment` | vLLM with GPU requests, health probes, security context |
| `Service` | Prometheus-annotated ClusterIP |
| `HorizontalPodAutoscaler` | Token-rate based autoscaling (when `max_replicas > min_replicas`) |
| `HTTPRoute` | Gateway API routing with model header matching |
| `InferenceModel` | Gateway API Inference Extension model registration |
| `NetworkPolicy` | Ingress restricted to gateway, egress to DNS + HTTPS only |

## Related projects

- [gpucast](https://github.com/amayabdaniel/gpucast) — inference cost tracking (cost/request, cost/model, cost/tenant)
- [modelgate](https://github.com/amayabdaniel/modelgate) — LLM security proxy (prompt injection, PII, audit trails)

## Tests

```bash
make test    # 38 tests
make build   # builds to bin/inferctl
```

## License

Apache 2.0
