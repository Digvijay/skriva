# Deploy to Azure Container Apps

Production deployment with HTTPS, custom domain, and zero server management. Costs ~$5-10/month.

## Architecture

```
Browser → Azure Container Apps (managed HTTPS) → Skriva Container → Azure Files
```

## Prerequisites

- [Azure CLI](https://docs.microsoft.com/en-us/cli/azure/install-azure-cli)
- An Azure subscription
- A custom domain (optional)

## Step 1: Set up resources

```bash
RESOURCE_GROUP="blog-rg"
LOCATION="eastus"
ENVIRONMENT="blog-env"
STORAGE_ACCOUNT="blogdata$(openssl rand -hex 4)"

az login
az group create --name $RESOURCE_GROUP --location $LOCATION

az storage account create \
  --name $STORAGE_ACCOUNT \
  --resource-group $RESOURCE_GROUP \
  --location $LOCATION \
  --sku Standard_LRS

STORAGE_KEY=$(az storage account keys list \
  --account-name $STORAGE_ACCOUNT \
  --resource-group $RESOURCE_GROUP \
  --query '[0].value' -o tsv)

az storage share create --name blog-content --account-name $STORAGE_ACCOUNT
az storage share create --name blog-config  --account-name $STORAGE_ACCOUNT
```

## Step 2: Upload config

```bash
az storage file upload \
  --share-name blog-config \
  --source data/config/site.yaml \
  --account-name $STORAGE_ACCOUNT

az storage file upload \
  --share-name blog-config \
  --source data/config/secrets.yaml \
  --account-name $STORAGE_ACCOUNT
```

## Step 3: Deploy

```bash
az containerapp env create \
  --name $ENVIRONMENT \
  --resource-group $RESOURCE_GROUP \
  --location $LOCATION

az containerapp create \
  --name blog \
  --resource-group $RESOURCE_GROUP \
  --environment $ENVIRONMENT \
  --image ghcr.io/digvijay/skriva:latest \
  --target-port 8080 \
  --ingress external \
  --min-replicas 1 --max-replicas 1 \
  --cpu 0.25 --memory 0.5Gi
```

::: tip
Add volume mounts via the Azure Portal: Container App → Containers → Volume mounts.
:::

## Step 4: Custom domain

```bash
az containerapp hostname add --name blog \
  --resource-group $RESOURCE_GROUP \
  --hostname blog.yourdomain.com

az containerapp hostname bind --name blog \
  --resource-group $RESOURCE_GROUP \
  --hostname blog.yourdomain.com \
  --environment $ENVIRONMENT \
  --validation-method CNAME
```

Add a CNAME record: `blog → <app>.azurecontainerapps.io`

## Cost

| Resource                   | Monthly          |
| -------------------------- | ---------------- |
| Container Apps (0.25 vCPU) | ~$5-10           |
| Azure Files (1 GB)         | ~$0.05           |
| Custom domain + HTTPS      | Free             |
| **Total**                  | **~$5-10/month** |
