# Service DNS Example

Demonstrates creating DNS records from a Kubernetes Service of type LoadBalancer.

## Prerequisites

- ExternalDNS with HuaweiCloud webhook deployed (see [basic-deployment](../basic-deployment/))
- A DNS zone `external-dns.com` created in HuaweiCloud DNS
- A HuaweiCloud ELB instance

## Setup

1. Edit `service.yaml` — set `kubernetes.io/elb.id` to your ELB ID.
2. Apply:

```bash
kubectl apply -f service.yaml
```

## What It Does

- Creates an nginx Deployment and LoadBalancer Service
- The `external-dns.alpha.kubernetes.io/hostname` annotation tells ExternalDNS to create a DNS record
- ExternalDNS creates an A record and a TXT record for `nginx.external-dns.com`

## Expected Output

```bash
# Check DNS records in HuaweiCloud console or via:
kubectl logs -l app=external-dns -c external-dns
# Should show: "Created A record named 'nginx.external-dns.com'"
```
