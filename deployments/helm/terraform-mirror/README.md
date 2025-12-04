# Terraform Mirror Helm Chart

A Helm chart for deploying Terraform Mirror to Kubernetes.

## Prerequisites

- Kubernetes 1.24+
- Helm 3.10+
- PV provisioner support in the cluster (if persistence is enabled)
- S3-compatible storage (AWS S3, MinIO, etc.) or local storage

## Installation

### Quick Start

```bash
helm install terraform-mirror ./terraform-mirror \
  --namespace terraform-mirror \
  --create-namespace \
  --set secrets.jwtSecret="your-jwt-secret-at-least-32-chars" \
  --set secrets.s3AccessKey="your-access-key" \
  --set secrets.s3SecretKey="your-secret-key" \
  --set config.storage.s3.endpoint="https://s3.amazonaws.com" \
  --set config.storage.s3.bucket="your-bucket" \
  --set admin.password="your-admin-password"
```

### With Custom Values File

1. Create a `values.yaml` file:

```yaml
secrets:
  jwtSecret: "your-jwt-secret-at-least-32-characters"
  s3AccessKey: "your-access-key"
  s3SecretKey: "your-secret-key"

config:
  storage:
    type: s3
    s3:
      endpoint: "https://s3.amazonaws.com"
      bucket: "terraform-mirror"
      region: "us-east-1"

admin:
  createUser: true
  username: admin
  password: "secure-admin-password"

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: terraform-mirror.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: terraform-mirror-tls
      hosts:
        - terraform-mirror.example.com
```

2. Install:

```bash
helm install terraform-mirror ./terraform-mirror \
  --namespace terraform-mirror \
  --create-namespace \
  --values values.yaml
```

### With MinIO (All-in-One)

```yaml
minio:
  enabled: true
  rootUser: minioadmin
  rootPassword: minioadmin123

secrets:
  jwtSecret: "your-jwt-secret-at-least-32-characters"

admin:
  createUser: true
  password: "admin123"
```

## Configuration

### Required Values

| Parameter | Description |
|-----------|-------------|
| `secrets.jwtSecret` | JWT secret for authentication (min 32 chars) |
| `secrets.s3AccessKey` | S3 access key (required for S3 storage) |
| `secrets.s3SecretKey` | S3 secret key (required for S3 storage) |

### Common Values

| Parameter | Default | Description |
|-----------|---------|-------------|
| `replicaCount` | `1` | Number of replicas (1 recommended due to SQLite) |
| `image.repository` | `terraform-mirror` | Image repository |
| `image.tag` | `""` | Image tag (defaults to chart appVersion) |
| `config.storage.type` | `s3` | Storage type: `local` or `s3` |
| `config.storage.s3.endpoint` | `""` | S3 endpoint URL |
| `config.storage.s3.bucket` | `terraform-mirror` | S3 bucket name |
| `config.cache.enabled` | `true` | Enable caching |
| `config.cache.memoryMB` | `256` | Memory cache size in MB |
| `config.processor.workers` | `4` | Number of download workers |
| `persistence.enabled` | `true` | Enable persistent storage |
| `persistence.size` | `20Gi` | PVC size |
| `ingress.enabled` | `false` | Enable ingress |
| `admin.createUser` | `true` | Create admin user on install |
| `admin.username` | `admin` | Admin username |
| `admin.password` | `""` | Admin password |

### Using Existing Secrets

Instead of creating secrets via Helm:

```yaml
secrets:
  existingSecret: "my-existing-secret"
  existingSecretKeys:
    jwtSecret: "jwt-secret"
    s3AccessKey: "s3-access-key"
    s3SecretKey: "s3-secret-key"
```

### Resource Sizing

Small deployment (< 50 providers):
```yaml
resources:
  requests:
    memory: "256Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"

config:
  cache:
    memoryMB: 128
    diskMaxMB: 2048
```

Large deployment (200+ providers):
```yaml
resources:
  requests:
    memory: "1Gi"
    cpu: "500m"
  limits:
    memory: "2Gi"
    cpu: "2000m"

config:
  cache:
    memoryMB: 1024
    diskMaxMB: 20480
  processor:
    workers: 8
```

## Upgrading

```bash
helm upgrade terraform-mirror ./terraform-mirror \
  --namespace terraform-mirror \
  --values values.yaml
```

## Uninstalling

```bash
helm uninstall terraform-mirror --namespace terraform-mirror
```

**Note:** PVCs are not deleted automatically. To remove:

```bash
kubectl delete pvc -n terraform-mirror terraform-mirror-data
```

## Troubleshooting

### Check pod status

```bash
kubectl get pods -n terraform-mirror
kubectl describe pod -n terraform-mirror <pod-name>
kubectl logs -n terraform-mirror <pod-name>
```

### Check configuration

```bash
kubectl get configmap -n terraform-mirror terraform-mirror-config -o yaml
```

### Create admin user manually

```bash
kubectl exec -it -n terraform-mirror deployment/terraform-mirror -- \
  /app/create-admin -config /app/config.hcl -username admin -password your-password
```

### Test connectivity

```bash
kubectl port-forward -n terraform-mirror svc/terraform-mirror 8080:8080
curl http://localhost:8080/health
```
