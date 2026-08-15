# Kubernetes Examples

Kubernetes manifests for deploying OpenSQS to a cluster. These are plain YAML manifests — for production deployments, use the [Helm chart](../../deploy/helm/) instead.

## Available Examples

### 1. No Authentication (`k8s.no-auth.yaml`)

Deploys OpenSQS with `auth.enabled: false`. Any pod in the cluster can send and receive messages without credentials.

**Use when:** Development clusters, testing, internal tooling where auth isn't needed.

```bash
kubectl apply -f examples/kubernetes/k8s.no-auth.yaml

# Access via port-forward
kubectl port-forward svc/opensqs 9324:9324 -n opensqs

# Test (no credentials needed)
export AWS_ENDPOINT_URL=http://localhost:9324
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_DEFAULT_REGION=us-east-1
aws sqs create-queue --queue-name my-queue
```

### 2. With Initial Credentials (`k8s.with-creds.yaml` + `k8s.secret.yaml`)

Deploys OpenSQS with authentication enabled and pre-seeded credentials. The access key / secret key are stored in a Kubernetes Secret and injected as environment variables.

**Use when:** Staging environments, teams that need authenticated access from the first boot.

```bash
# 1. Create the namespace and secret first
kubectl apply -f examples/kubernetes/k8s.secret.yaml

# 2. Deploy OpenSQS
kubectl apply -f examples/kubernetes/k8s.with-creds.yaml

# Access via port-forward
kubectl port-forward svc/opensqs 9324:9324 -n opensqs

# Test with pre-seeded credentials
export AWS_ENDPOINT_URL=http://localhost:9324
export AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE
export AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
export AWS_DEFAULT_REGION=us-east-1
aws sqs create-queue --queue-name my-queue
```

### 3. Persistent Storage (`k8s.persistent.yaml` + `k8s.secret.yaml`)

Deploys OpenSQS with SQLite persistent storage (PVC) and pre-seeded credentials. Messages and queues survive pod restarts.

**Use when:** Production or staging where data durability is required.

```bash
# 1. Create the secret first
kubectl apply -f examples/kubernetes/k8s.secret.yaml

# 2. Deploy with persistence
kubectl apply -f examples/kubernetes/k8s.persistent.yaml

# Access via port-forward
kubectl port-forward svc/opensqs 9324:9324 -n opensqs
```

## Files

| File | Description |
|------|-------------|
| `k8s.no-auth.yaml` | Namespace, ConfigMap, Deployment, Service — no auth |
| `k8s.with-creds.yaml` | Namespace, ConfigMap, Deployment, Service — with auth |
| `k8s.persistent.yaml` | Same as above + PVC for SQLite persistence |
| `k8s.secret.yaml` | Kubernetes Secret with server secret and credentials |

## How Credentials Work

The `k8s.with-creds.yaml` and `k8s.persistent.yaml` manifests inject initial credentials via environment variables that override the config file:

```
SQS__SERVERSECRET                           → sqs.serverSecret
SQS__AUTH__INITIALCREDENTIALS__0__LABEL      → auth.initialCredentials[0].label
SQS__AUTH__INITIALCREDENTIALS__0__ACCESSKEYID → auth.initialCredentials[0].accessKeyId
SQS__AUTH__INITIALCREDENTIALS__0__SECRETACCESSKEY → auth.initialCredentials[0].secretAccessKey
```

The double-underscore (`__`) notation maps to nested YAML keys. The `0` index selects the first entry in the `initialCredentials` list.

## Customizing

### Using Your Own Credentials

Edit `k8s.secret.yaml`:

```yaml
stringData:
  serverSecret: "your-strong-random-secret"
  credLabel: "my-team-key"
  accessKeyId: "YOUR_ACCESS_KEY_ID"
  secretAccessKey: "YOUR_SECRET_ACCESS_KEY"
```

### Using BadgerDB Instead of SQLite

Edit the ConfigMap in `k8s.persistent.yaml`:

```yaml
sqs:
  storageType: "badger"
  badgerPath: "/data/badger"
```

### Exposing Externally

To expose OpenSQS outside the cluster, add an Ingress:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: opensqs
  namespace: opensqs
spec:
  rules:
    - host: opensqs.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: opensqs
                port:
                  number: 9324
```

Or use a LoadBalancer service:

```bash
kubectl patch svc opensqs -n opensqs -p '{"spec":{"type":"LoadBalancer"}}'
```

## Production Recommendations

For production deployments, use the [Helm chart](../../deploy/helm/) instead of these raw manifests. The Helm chart includes:

- Configurable resource limits and requests
- Horizontal Pod Autoscaler (HPA)
- Pod Disruption Budget (PDB)
- Ingress with TLS
- Proper secret management
- Readiness/liveness/startup probes

```bash
helm install opensqs deploy/helm \
  --set image.tag=v0.0.7 \
  --set sqs.serverSecret="your-secret-key" \
  --set opensqs.sqs.storageType="badger" \
  --set persistence.enabled=true
```

## Cleanup

```bash
# Delete everything
kubectl delete namespace opensqs
```

## See Also

- [Helm Chart](../../deploy/helm/) — Production Kubernetes deployment
- [Configuration Reference](../../docs/configuration.md) — All config options
- [Docker Compose Examples](../docker-compose/) — Local container orchestration
