# Deployment Guide

This guide covers deploying Terraform Mirror in production environments using Docker Compose, Kubernetes, and Helm.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Deployment Options Overview](#deployment-options-overview)
- [Docker Compose Deployment](#docker-compose-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Helm Chart Deployment](#helm-chart-deployment)
- [Production Configuration](#production-configuration)
- [High Availability](#high-availability)
- [Monitoring](#monitoring)
- [Backup and Recovery](#backup-and-recovery)
- [Security Hardening](#security-hardening)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

- **Docker & Docker Compose**: v20.10+ for containerized deployments
- **Kubernetes**: v1.24+ for K8s deployments
- **Helm**: v3.10+ for Helm chart deployments
- **S3-Compatible Storage**: MinIO, AWS S3, or compatible service for production
- **TLS Certificates**: For HTTPS (recommended for production)

---

## Deployment Options Overview

| Option | Best For | Complexity | HA Support |
|--------|----------|------------|------------|
| Docker Compose | Small teams, dev/staging | Low | Limited |
| Kubernetes | Enterprise, production | Medium | Yes |
| Helm | Enterprise, GitOps | Medium | Yes |

---

## Docker Compose Deployment

### Basic Setup

1. **Create deployment directory:**
   ```bash
   mkdir -p /opt/terraform-mirror
   cd /opt/terraform-mirror
   ```

2. **Create configuration file** `config.hcl`:
   ```hcl
   server {
     port     = "8080"
     hostname = "0.0.0.0"
   }

   storage {
     type            = "s3"
     s3_endpoint     = "http://minio:9000"
     s3_bucket       = "terraform-mirror"
     s3_region       = "us-east-1"
     s3_access_key   = "${TFM_S3_ACCESS_KEY}"
     s3_secret_key   = "${TFM_S3_SECRET_KEY}"
     s3_use_path_style = true
   }

   database {
     path = "/data/mirror.db"
   }

   cache {
     enabled    = true
     memory_mb  = 256
     disk_enabled = true
     disk_path  = "/data/cache"
     disk_max_mb = 4096
   }

   features {
     multi_platform = true
     enable_admin   = true
   }

   auth {
     token_expiry = "24h"
     jwt_secret   = "${TFM_JWT_SECRET}"
   }

   processor {
     enabled          = true
     workers          = 4
     download_timeout = "15m"
   }

   logging {
     level  = "info"
     format = "json"
   }
   ```

3. **Create `docker-compose.yml`:**
   ```yaml
   version: '3.8'

   services:
     terraform-mirror:
       image: your-registry/terraform-mirror:latest
       restart: unless-stopped
       ports:
         - "8080:8080"
       volumes:
         - ./config.hcl:/app/config.hcl:ro
         - mirror-data:/data
       environment:
         - TFM_JWT_SECRET=${TFM_JWT_SECRET}
         - TFM_S3_ACCESS_KEY=${MINIO_ACCESS_KEY}
         - TFM_S3_SECRET_KEY=${MINIO_SECRET_KEY}
       depends_on:
         minio:
           condition: service_healthy
       healthcheck:
         test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
         interval: 30s
         timeout: 10s
         retries: 3

     minio:
       image: minio/minio:latest
       restart: unless-stopped
       command: server /data --console-address ":9001"
       ports:
         - "9000:9000"
         - "9001:9001"
       volumes:
         - minio-data:/data
       environment:
         - MINIO_ROOT_USER=${MINIO_ACCESS_KEY}
         - MINIO_ROOT_PASSWORD=${MINIO_SECRET_KEY}
       healthcheck:
         test: ["CMD", "curl", "-f", "http://localhost:9000/minio/health/live"]
         interval: 30s
         timeout: 10s
         retries: 3

     minio-init:
       image: minio/mc:latest
       depends_on:
         minio:
           condition: service_healthy
       entrypoint: >
         /bin/sh -c "
         mc alias set minio http://minio:9000 ${MINIO_ACCESS_KEY} ${MINIO_SECRET_KEY};
         mc mb minio/terraform-mirror --ignore-existing;
         exit 0;
         "

   volumes:
     mirror-data:
     minio-data:
   ```

4. **Create `.env` file:**
   ```bash
   MINIO_ACCESS_KEY=your-access-key
   MINIO_SECRET_KEY=your-very-long-secret-key-at-least-32-chars
   TFM_JWT_SECRET=your-jwt-secret-at-least-32-characters
   ```

5. **Deploy:**
   ```bash
   docker-compose up -d
   ```

6. **Create admin user:**
   ```bash
   docker-compose exec terraform-mirror /app/create-admin \
     -config /app/config.hcl \
     -username admin \
     -password your-secure-password
   ```

### With Nginx Reverse Proxy

Add nginx for TLS termination:

```yaml
services:
  nginx:
    image: nginx:alpine
    restart: unless-stopped
    ports:
      - "443:443"
      - "80:80"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro
      - ./certs:/etc/nginx/certs:ro
    depends_on:
      - terraform-mirror
```

Example `nginx.conf`:
```nginx
events {
    worker_connections 1024;
}

http {
    upstream mirror {
        server terraform-mirror:8080;
    }

    server {
        listen 80;
        return 301 https://$host$request_uri;
    }

    server {
        listen 443 ssl;
        
        ssl_certificate /etc/nginx/certs/server.crt;
        ssl_certificate_key /etc/nginx/certs/server.key;
        ssl_protocols TLSv1.2 TLSv1.3;
        
        location / {
            proxy_pass http://mirror;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
            proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
            proxy_set_header X-Forwarded-Proto $scheme;
        }
    }
}
```

---

## Kubernetes Deployment

### Namespace and ConfigMap

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: terraform-mirror
---
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: terraform-mirror-config
  namespace: terraform-mirror
data:
  config.hcl: |
    server {
      port     = "8080"
      hostname = "0.0.0.0"
    }

    storage {
      type            = "s3"
      s3_endpoint     = "http://minio.terraform-mirror.svc:9000"
      s3_bucket       = "terraform-mirror"
      s3_region       = "us-east-1"
      s3_use_path_style = true
    }

    database {
      path = "/data/mirror.db"
    }

    cache {
      enabled      = true
      memory_mb    = 512
      disk_enabled = true
      disk_path    = "/data/cache"
      disk_max_mb  = 8192
    }

    features {
      multi_platform = true
      enable_admin   = true
    }

    auth {
      token_expiry = "24h"
    }

    processor {
      enabled          = true
      workers          = 4
      download_timeout = "15m"
    }

    logging {
      level  = "info"
      format = "json"
    }
```

### Secrets

```yaml
# secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: terraform-mirror-secrets
  namespace: terraform-mirror
type: Opaque
stringData:
  s3-access-key: "your-access-key"
  s3-secret-key: "your-secret-key"
  jwt-secret: "your-jwt-secret-at-least-32-chars"
```

### Deployment

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: terraform-mirror
  namespace: terraform-mirror
  labels:
    app: terraform-mirror
spec:
  replicas: 1  # Single replica due to SQLite
  selector:
    matchLabels:
      app: terraform-mirror
  template:
    metadata:
      labels:
        app: terraform-mirror
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        fsGroup: 1000
      containers:
        - name: terraform-mirror
          image: your-registry/terraform-mirror:latest
          ports:
            - containerPort: 8080
          env:
            - name: TFM_S3_ACCESS_KEY
              valueFrom:
                secretKeyRef:
                  name: terraform-mirror-secrets
                  key: s3-access-key
            - name: TFM_S3_SECRET_KEY
              valueFrom:
                secretKeyRef:
                  name: terraform-mirror-secrets
                  key: s3-secret-key
            - name: TFM_JWT_SECRET
              valueFrom:
                secretKeyRef:
                  name: terraform-mirror-secrets
                  key: jwt-secret
          volumeMounts:
            - name: config
              mountPath: /app/config.hcl
              subPath: config.hcl
              readOnly: true
            - name: data
              mountPath: /data
          resources:
            requests:
              memory: "256Mi"
              cpu: "100m"
            limits:
              memory: "1Gi"
              cpu: "1000m"
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 10
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
      volumes:
        - name: config
          configMap:
            name: terraform-mirror-config
        - name: data
          persistentVolumeClaim:
            claimName: terraform-mirror-data
```

### PersistentVolumeClaim

```yaml
# pvc.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: terraform-mirror-data
  namespace: terraform-mirror
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 20Gi
  storageClassName: standard  # Adjust for your cluster
```

### Service

```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: terraform-mirror
  namespace: terraform-mirror
spec:
  selector:
    app: terraform-mirror
  ports:
    - port: 8080
      targetPort: 8080
  type: ClusterIP
```

### Ingress

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: terraform-mirror
  namespace: terraform-mirror
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/proxy-body-size: "500m"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - terraform-mirror.example.com
      secretName: terraform-mirror-tls
  rules:
    - host: terraform-mirror.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: terraform-mirror
                port:
                  number: 8080
```

### Deploy to Kubernetes

```bash
kubectl apply -f namespace.yaml
kubectl apply -f secrets.yaml
kubectl apply -f configmap.yaml
kubectl apply -f pvc.yaml
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f ingress.yaml

# Create admin user
kubectl exec -it -n terraform-mirror deployment/terraform-mirror -- \
  /app/create-admin -config /app/config.hcl -username admin -password secure-password
```

---

## Helm Chart Deployment

### Using the Helm Chart

The Helm chart is located in `deployments/helm/terraform-mirror/`.

```bash
# Install from local chart
helm install terraform-mirror ./deployments/helm/terraform-mirror \
  --namespace terraform-mirror \
  --create-namespace \
  --values values.yaml

# Or if published to a registry
helm repo add terraform-mirror https://charts.example.com
helm install terraform-mirror terraform-mirror/terraform-mirror \
  --namespace terraform-mirror \
  --create-namespace \
  --values values.yaml
```

### Example `values.yaml`

```yaml
replicaCount: 1

image:
  repository: your-registry/terraform-mirror
  tag: latest
  pullPolicy: Always

config:
  server:
    port: "8080"
  storage:
    type: s3
    s3Endpoint: "http://minio:9000"
    s3Bucket: "terraform-mirror"
    s3Region: "us-east-1"
    s3UsePathStyle: true
  database:
    path: "/data/mirror.db"
  cache:
    enabled: true
    memoryMB: 512
    diskEnabled: true
    diskPath: "/data/cache"
    diskMaxMB: 8192
  features:
    multiPlatform: true
    enableAdmin: true
  processor:
    enabled: true
    workers: 4
    downloadTimeout: "15m"
  logging:
    level: info
    format: json

secrets:
  s3AccessKey: "your-access-key"
  s3SecretKey: "your-secret-key"
  jwtSecret: "your-jwt-secret-at-least-32-chars"

persistence:
  enabled: true
  size: 20Gi
  storageClass: standard

service:
  type: ClusterIP
  port: 8080

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

resources:
  requests:
    memory: "256Mi"
    cpu: "100m"
  limits:
    memory: "1Gi"
    cpu: "1000m"

minio:
  enabled: true
  persistence:
    size: 100Gi
```

### Helm Chart Structure

```
helm/terraform-mirror/
├── Chart.yaml
├── values.yaml
├── templates/
│   ├── _helpers.tpl
│   ├── configmap.yaml
│   ├── deployment.yaml
│   ├── ingress.yaml
│   ├── pvc.yaml
│   ├── secrets.yaml
│   └── service.yaml
└── charts/
    └── minio/  # Optional subchart
```

---

## Production Configuration

### Recommended Settings

```hcl
server {
  port     = "8080"
  hostname = "0.0.0.0"
}

storage {
  type            = "s3"
  s3_endpoint     = "https://s3.amazonaws.com"  # Or your S3-compatible endpoint
  s3_bucket       = "your-terraform-mirror-bucket"
  s3_region       = "us-east-1"
  s3_access_key   = "${TFM_S3_ACCESS_KEY}"
  s3_secret_key   = "${TFM_S3_SECRET_KEY}"
  s3_use_path_style = false  # True for MinIO, false for AWS S3
}

database {
  path              = "/data/mirror.db"
  backup_enabled    = true
  backup_interval_hours = 6
  backup_to_s3      = true
  backup_s3_prefix  = "backups/"
}

cache {
  enabled      = true
  memory_mb    = 512         # Adjust based on available RAM
  ttl_minutes  = 60
  disk_enabled = true
  disk_path    = "/data/cache"
  disk_max_mb  = 10240       # 10GB disk cache
  disk_ttl_minutes = 1440    # 24 hours
}

features {
  multi_platform = true
  enable_admin   = true
}

auth {
  token_expiry = "8h"        # Shorter for security
  jwt_secret   = "${TFM_JWT_SECRET}"
}

processor {
  enabled          = true
  workers          = 8       # Adjust based on CPU cores
  download_timeout = "20m"
  retry_attempts   = 3
  retry_delay      = "30s"
}

logging {
  level  = "info"
  format = "json"
}

telemetry {
  enabled = true
  prometheus_path = "/metrics"
}
```

### Environment Variables (Production)

```bash
# Required
TFM_JWT_SECRET=<32+ character random string>
TFM_S3_ACCESS_KEY=<your-s3-access-key>
TFM_S3_SECRET_KEY=<your-s3-secret-key>

# Optional overrides
TFM_SERVER_PORT=8080
TFM_LOGGING_LEVEL=info
TFM_CACHE_MEMORY_MB=1024
```

### Resource Sizing

| Workload | CPU | Memory | Cache (Memory) | Cache (Disk) |
|----------|-----|--------|----------------|--------------|
| Small (<50 providers) | 1 core | 512MB | 128MB | 2GB |
| Medium (50-200 providers) | 2 cores | 1GB | 512MB | 10GB |
| Large (200+ providers) | 4 cores | 2GB | 1GB | 50GB |

---

## High Availability

### Limitations

Terraform Mirror uses SQLite, which limits horizontal scaling. For HA:

1. **Single active instance** - Only one instance writes to the database
2. **Shared storage** - Use network-attached storage for the data volume
3. **Load balancer** - Front with a load balancer for health checks

### Recommended HA Architecture

```
                    ┌─────────────────┐
                    │  Load Balancer  │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
    ┌─────────▼──────┐  (standby)  ┌───────▼────────┐
    │   Primary      │             │    Secondary   │
    │   Instance     │             │    Instance    │
    └───────┬────────┘             └───────┬────────┘
            │                              │
            │         ┌────────────────────┤
            │         │                    │
    ┌───────▼─────────▼──────┐    ┌───────▼────────┐
    │   Shared NFS/EFS       │    │   S3 Storage   │
    │   (Database + Cache)   │    │   (Providers)  │
    └────────────────────────┘    └────────────────┘
```

### Future: PostgreSQL Support

For true HA with multiple active replicas, PostgreSQL support is planned. This will enable:
- Multiple read replicas
- Active-active configuration
- Better scalability

---

## Monitoring

### Health Checks

**Liveness probe:**
```bash
curl http://localhost:8080/health
# Returns: {"status": "healthy"}
```

**Readiness probe:**
```bash
curl http://localhost:8080/health
```

### Prometheus Metrics

Enable metrics in configuration:
```hcl
telemetry {
  enabled = true
  prometheus_path = "/metrics"
}
```

Access metrics:
```bash
curl http://localhost:8080/metrics
```

### Key Metrics to Monitor

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `http_requests_total` | Total HTTP requests | N/A |
| `http_request_duration_seconds` | Request latency | P99 > 5s |
| `cache_hits_total` | Cache hits | N/A |
| `cache_misses_total` | Cache misses | Hit rate < 70% |
| `jobs_pending` | Pending jobs | > 100 |
| `jobs_failed_total` | Failed jobs | Any increase |
| `storage_bytes_total` | Storage usage | > 80% capacity |

### Grafana Dashboard

Example dashboard queries:

**Request rate:**
```promql
rate(http_requests_total[5m])
```

**Cache hit rate:**
```promql
rate(cache_hits_total[5m]) / (rate(cache_hits_total[5m]) + rate(cache_misses_total[5m]))
```

**P99 latency:**
```promql
histogram_quantile(0.99, rate(http_request_duration_seconds_bucket[5m]))
```

### Logging

Configure JSON logging for production:
```hcl
logging {
  level  = "info"
  format = "json"
}
```

Example log output:
```json
{"level":"info","ts":"2024-01-15T10:30:00Z","msg":"request completed","method":"GET","path":"/v1/providers/hashicorp/aws/versions","status":200,"duration_ms":15}
```

Integrate with:
- **ELK Stack** (Elasticsearch, Logstash, Kibana)
- **Loki** (Grafana)
- **CloudWatch Logs** (AWS)
- **Stackdriver** (GCP)

---

## Backup and Recovery

### Automated Backups

Enable in configuration:
```hcl
database {
  backup_enabled        = true
  backup_interval_hours = 6
  backup_to_s3          = true
  backup_s3_prefix      = "backups/"
}
```

### Manual Backup

**Via API:**
```bash
curl -X POST http://localhost:8080/admin/api/backup \
  -H "Authorization: Bearer $TOKEN"
```

**Via Docker:**
```bash
# Stop the container (for consistency)
docker-compose stop terraform-mirror

# Copy database
docker cp terraform-mirror:/data/mirror.db ./backup-$(date +%Y%m%d).db

# Restart
docker-compose start terraform-mirror
```

**Via Kubernetes:**
```bash
kubectl exec -n terraform-mirror deployment/terraform-mirror -- \
  cp /data/mirror.db /data/backup-$(date +%Y%m%d).db
```

### Recovery

1. **Stop the service**
2. **Restore database:**
   ```bash
   cp backup.db /data/mirror.db
   ```
3. **Verify integrity:**
   ```bash
   sqlite3 /data/mirror.db "PRAGMA integrity_check;"
   ```
4. **Start the service**

### Disaster Recovery

For complete disaster recovery:

1. **Database**: Restore from S3 backup or manual backup
2. **Storage**: Providers are in S3 (durable)
3. **Configuration**: Store config in version control
4. **Secrets**: Use a secrets manager (Vault, AWS Secrets Manager)

---

## Security Hardening

### Network Security

1. **Use HTTPS** - Always use TLS in production
2. **Restrict access** - Firewall rules for admin endpoints
3. **Private network** - Deploy in private subnet if possible

### Authentication

1. **Strong passwords** - Enforce minimum complexity
2. **Rotate JWT secret** - Change periodically
3. **Short token expiry** - Use 8h or less for production

### Container Security

```yaml
# Pod security context
securityContext:
  runAsNonRoot: true
  runAsUser: 1000
  fsGroup: 1000
  readOnlyRootFilesystem: true
```

### Secrets Management

Don't store secrets in:
- Configuration files
- Docker images
- Git repositories

Use:
- Kubernetes Secrets (encrypted at rest)
- HashiCorp Vault
- AWS Secrets Manager
- Azure Key Vault

### Audit Logging

Review audit logs regularly:
```bash
# Via API
curl http://localhost:8080/admin/api/stats/audit \
  -H "Authorization: Bearer $TOKEN"
```

### Security Checklist

- [ ] HTTPS enabled with valid certificates
- [ ] Default admin password changed
- [ ] JWT secret is strong (32+ chars, random)
- [ ] S3 credentials rotated regularly
- [ ] Container runs as non-root
- [ ] Network policies restrict traffic
- [ ] Audit logs monitored
- [ ] Backups encrypted
- [ ] Secrets in secure storage

---

## Troubleshooting

### Common Issues

#### Container Won't Start

**Symptom:** Container exits immediately

**Check:**
```bash
docker logs terraform-mirror
kubectl logs -n terraform-mirror deployment/terraform-mirror
```

**Common causes:**
- Invalid configuration file
- Missing environment variables
- Permission issues on data directory

#### Can't Connect to S3

**Symptom:** "connection refused" or "access denied"

**Check:**
```bash
# Test S3 connectivity
curl -v http://minio:9000/minio/health/live
```

**Solutions:**
- Verify endpoint URL
- Check access key and secret
- Ensure bucket exists
- Check network policies

#### Database Locked

**Symptom:** "database is locked" errors

**Cause:** Multiple processes accessing SQLite

**Solutions:**
- Ensure only one replica
- Check for orphaned processes
- Verify volume isn't shared incorrectly

#### High Memory Usage

**Symptom:** Container OOM killed

**Solutions:**
- Reduce cache memory limit
- Increase container memory limit
- Check for memory leaks (report bug)

#### Slow Downloads

**Symptom:** Provider downloads take too long

**Check:**
- Network latency to public registry
- S3 storage performance
- Cache hit rate

**Solutions:**
- Increase cache sizes
- Optimize network path
- Increase worker count

### Debug Mode

Enable debug logging:
```hcl
logging {
  level = "debug"
}
```

Or via environment:
```bash
TFM_LOGGING_LEVEL=debug
```

### Support Information

When reporting issues, include:
- Version: `terraform-mirror --version`
- Configuration (redact secrets)
- Relevant logs
- Steps to reproduce
