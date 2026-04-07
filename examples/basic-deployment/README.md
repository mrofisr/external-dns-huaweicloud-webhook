# Basic Deployment (IDP Token Auth)

Deploys ExternalDNS with the HuaweiCloud webhook sidecar using IDP token authentication via a projected ServiceAccountToken.

## Prerequisites

- A Kubernetes cluster on HuaweiCloud (CCE)
- An identity provider configured in HuaweiCloud IAM ([guide](https://support.huaweicloud.com/intl/en-us/bestpractice-cce/cce_bestpractice_0333.html))
- A DNS zone created in HuaweiCloud DNS

## Setup

1. Edit `configmap.yaml` — set your `region`, `projectId`, and `idpId`.
2. Apply the manifests:

```bash
kubectl apply -f configmap.yaml
kubectl apply -f rbac.yaml
kubectl apply -f deployment.yaml
```

## What It Does

- Creates a ConfigMap with HuaweiCloud credentials configuration
- Sets up RBAC (ServiceAccount, ClusterRole, ClusterRoleBinding)
- Deploys ExternalDNS with the HuaweiCloud webhook as a sidecar container
- The webhook listens on port 8888 and manages DNS records via HuaweiCloud API

## Expected Output

```
kubectl logs -l app=external-dns -c huaweicloud-webhook
time="..." level=info msg="Starting server on localhost:8888"
time="..." level=info msg="Retrieving HuaweiCloud DNS domain records"
```
