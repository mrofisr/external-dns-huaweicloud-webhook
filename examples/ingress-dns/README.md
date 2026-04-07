# Ingress DNS Example

Demonstrates creating DNS records from a Kubernetes Ingress resource.

## Prerequisites

- ExternalDNS with HuaweiCloud webhook deployed (see [basic-deployment](../basic-deployment/))
- A DNS zone `external-dns.com` created in HuaweiCloud DNS
- A HuaweiCloud ELB instance with an Ingress controller

## Setup

1. Edit `ingress.yaml` — set `kubernetes.io/elb.id` to your ELB ID.
2. Apply:

```bash
kubectl apply -f ingress.yaml
```

## What It Does

- Creates an nginx Deployment, NodePort Service, and Ingress
- The Ingress `host` field tells ExternalDNS to create a DNS record for `nginx.external-dns.com`
- ExternalDNS creates an A record and a TXT record

## Expected Output

```bash
kubectl logs -l app=external-dns -c external-dns
# Should show: "Created A record named 'nginx.external-dns.com'"
```
