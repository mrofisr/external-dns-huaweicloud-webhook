# ExternalDNS HuaweiCloud Webhook

ExternalDNS webhook provider for [HuaweiCloud DNS](https://www.huaweicloud.com/intl/en-us/product/dns.html). Manages public and private DNS zones in HuaweiCloud from Kubernetes using the [ExternalDNS](https://github.com/kubernetes-sigs/external-dns) webhook protocol.

## Requirements

- Go 1.26+
- Kubernetes cluster (HuaweiCloud CCE recommended)
- HuaweiCloud account with DNS permissions
- [ExternalDNS](https://github.com/kubernetes-sigs/external-dns) v0.21.0+

## Dependencies

| Module | Version |
|--------|---------|
| `sigs.k8s.io/external-dns` | v0.21.0 |
| `github.com/huaweicloud/huaweicloud-sdk-go-v3` | v0.1.191 |

All other functionality uses the Go standard library (`log/slog`, `net/http`, `encoding/json`, `os`).

## Project Structure

```
├── cmd/webhook/
│   ├── main.go                          # Entry point
│   └── init/
│       ├── configuration/               # Env-based config parsing (stdlib)
│       ├── logging/                     # log/slog setup
│       └── server/                      # HTTP server with net/http.ServeMux
├── internal/dnsprovider/
│   ├── api.go                           # HuaweiCloudDNSAPI interface
│   └── dnsprovider.go                   # Core DNS provider implementation
├── pkg/webhook/
│   └── webhook.go                       # HTTP handlers for ExternalDNS protocol
├── examples/
│   ├── basic-deployment/                # IDP token auth deployment
│   ├── static-credentials/              # AK/SK auth deployment
│   ├── service-dns/                     # DNS from LoadBalancer Service
│   └── ingress-dns/                     # DNS from Ingress
├── Dockerfile                           # Multi-stage build (Go 1.26 + Alpine 3.21)
├── Makefile                             # Build, lint, docker targets
├── .golangci.yml                        # Linter configuration
└── go.mod
```

## Authentication

Two authentication modes are supported:

### IDP Token (recommended for CCE)

Uses a projected ServiceAccountToken exchanged for a HuaweiCloud scoped token via IAM. Configure an identity provider in HuaweiCloud IAM ([guide](https://support.huaweicloud.com/intl/en-us/bestpractice-cce/cce_bestpractice_0333.html)).

```yaml
# ConfigMap
data:
  cred.json: |
    {
      "region": "ap-southeast-3",
      "projectId": "<your-project-id>",
      "idpId": "<your-idp-name>"
    }
```

### Static Credentials (AK/SK)

```yaml
data:
  cred.json: |
    {
      "region": "ap-southeast-3",
      "accessKey": "<your-access-key>",
      "secretKey": "<your-secret-key>"
    }
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `CONFIG_FILE` | `/etc/kubernetes/huawei-cloud.json` | Path to HuaweiCloud JSON config |
| `ZONE_TYPE` | `public` | `public` or `private` |
| `TOKEN_FILE` | *(empty)* | Path to ServiceAccountToken (enables IDP auth) |
| `ZONE_MATCH_PARENT` | `false` | Match parent domain zones |
| `EXPIRATION_SECONDS` | `7200` | Token expiration time in seconds |
| `SERVER_HOST` | `localhost` | Server bind host |
| `SERVER_PORT` | `8888` | Server bind port |
| `DOMAIN_FILTER` | *(empty)* | Comma-separated domain filter |
| `EXCLUDE_DOMAINS` | *(empty)* | Comma-separated domain exclusions |
| `ZONE_ID_FILTER` | *(empty)* | Comma-separated zone ID filter |
| `DRY_RUN` | `false` | Enable dry-run mode |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `LOG_FORMAT` | `text` | Log format (`text` or `json`) |

## Build

```bash
# Build binary
make build

# Run locally
make run

# Format, vet, lint
make fmt
make vet
make lint

# Tidy modules
make tidy
```

## Docker

```bash
# Build image
make docker-build

# Build with custom registry/tag
REGISTRY=swr.ap-southeast-3.myhuaweicloud.com/myns IMAGE_TAG=v2.0.0 make docker-build

# Push
make docker-push
```

The Dockerfile uses a multi-stage build with `golang:1.26-alpine` and `alpine:3.21`.

## Linting

The project uses [golangci-lint](https://golangci-lint.run/) with configuration in `.golangci.yml`.

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linter
make lint
```

Enabled linters: `errcheck`, `govet`, `staticcheck`, `unused`, `gosimple`, `gofmt`, `goimports`, `misspell`, `unconvert`, `bodyclose`, `nilerr`, `errorlint`.

## Kubernetes Deployment

See the [examples/](examples/) directory for complete deployment manifests:

- **[basic-deployment](examples/basic-deployment/)** — Full deployment with IDP token auth, RBAC, and ConfigMap
- **[static-credentials](examples/static-credentials/)** — Deployment using AK/SK credentials
- **[service-dns](examples/service-dns/)** — Create DNS records from a LoadBalancer Service
- **[ingress-dns](examples/ingress-dns/)** — Create DNS records from an Ingress resource

### Quick Start

```bash
# 1. Create ConfigMap with credentials
kubectl apply -f examples/basic-deployment/configmap.yaml

# 2. Create RBAC resources
kubectl apply -f examples/basic-deployment/rbac.yaml

# 3. Deploy ExternalDNS with webhook
kubectl apply -f examples/basic-deployment/deployment.yaml

# 4. Verify
kubectl logs -l app=external-dns -c huaweicloud-webhook
```

## Webhook Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Negotiate — returns domain filter |
| GET | `/records` | List current DNS records |
| POST | `/records` | Apply DNS changes (create/update/delete) |
| POST | `/adjustendpoints` | Adjust endpoints |
| GET | `/health` | Health check |

> **Note:** The HuaweiCloud webhook does not currently support `alias` annotations.

## License

Apache License 2.0 — see [LICENSE](LICENSE).
