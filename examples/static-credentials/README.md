# Static Credentials

Deploys ExternalDNS with the HuaweiCloud webhook using static AccessKey/SecretKey authentication.

## Prerequisites

- A Kubernetes cluster
- HuaweiCloud AccessKey and SecretKey with DNS permissions
- A DNS zone created in HuaweiCloud DNS

## Setup

1. Create a Kubernetes Secret with your credentials:

```bash
kubectl create secret generic huaweicloud-credentials \
  --from-literal=accessKey=<your-ak> \
  --from-literal=secretKey=<your-sk>
```

2. Edit `configmap.yaml` — set your `region`.
3. Apply the manifests:

```bash
kubectl apply -f configmap.yaml
kubectl apply -f deployment.yaml
```

## What It Does

- Uses static AK/SK credentials mounted from a Secret
- No IDP token or ServiceAccountToken needed
- Suitable for non-CCE clusters or testing environments

## Expected Output

```
kubectl logs -l app=external-dns -c huaweicloud-webhook
time="..." level=info msg="Starting server on localhost:8888"
time="..." level=debug msg="using static credentials"
```
