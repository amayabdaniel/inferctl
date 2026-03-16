# inferctl

One manifest from local Ollama to Kubernetes vLLM.

The local-to-cloud AI deployment bridge that doesn't exist yet.

## The problem

165K developers use Ollama locally. 73K use vLLM in production. There is no tool that keeps a single config consistent across both. Teams manually rewrite configs, Helm charts, and deployment scripts every time they promote a model from laptop to cluster.

## What inferctl does

A CLI that reads a single `model.yaml` and:

- **Locally:** runs the model via Ollama with the right settings
- **On K8s:** generates manifests for vLLM + Gateway API Inference Extension routes + autoscaling + observability hooks

```yaml
# model.yaml
name: support-chat
model: qwen3:8b
context_length: 8192
quantization: q4_k_m
prompt_template: |
  You are a helpful customer support agent for {{company}}.
tools:
  - name: lookup_order
    endpoint: http://orders-api/v1/lookup
observability:
  metrics: true
  tracing: true
scaling:
  min_replicas: 1
  max_replicas: 4
  target_tokens_per_second: 500
```

```bash
# Run locally
inferctl dev

# Generate K8s manifests
inferctl gen --target vllm --output ./k8s/

# Deploy to cluster
inferctl apply --context prod-cluster
```

## Status

Early development. See `projectz/potential-projectz.md` for full plan.

## License

Apache 2.0
