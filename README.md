# azure-login

A lightweight, statically-linked Go CLI tool for Azure authentication in CI/CD environments. Drop-in replacement for Azure CLI authentication commands in GitHub Actions workflows.

## Features

- **Fast** - Significantly faster than Python-based Azure CLI
- **Lightweight** - ~10MB binary vs 1GB+ Azure CLI installation
- **Statically linked** - Single binary with no external dependencies
- **OIDC authentication** - Native support for GitHub Actions workload identity

## Quick Start

### Installation

```bash
# Linux AMD64
curl -L https://github.com/cogna-public/azure-login/releases/latest/download/azure-login-linux-amd64 -o azure-login
chmod +x azure-login
sudo mv azure-login /usr/local/bin/

# macOS ARM64 (Apple Silicon)
curl -L https://github.com/cogna-public/azure-login/releases/latest/download/azure-login-darwin-arm64 -o azure-login
chmod +x azure-login
sudo mv azure-login /usr/local/bin/
```

### GitHub Actions Usage

```yaml
permissions:
  id-token: write
  contents: read

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - name: Azure Login
        run: |
          azure-login login \
            --client-id ${{ vars.AZURE_CLIENT_ID }} \
            --tenant-id ${{ vars.AZURE_TENANT_ID }} \
            --subscription-id ${{ vars.AZURE_SUBSCRIPTION_ID }}

      - name: Get Access Token
        id: token
        run: |
          echo "token=$(azure-login account get-access-token --query accessToken -o tsv)" >> $GITHUB_OUTPUT

      - name: Login to ACR
        run: |
          azure-login acr login --name myregistry

      - name: Get AKS Credentials
        run: |
          azure-login aks get-credentials \
            --resource-group my-rg \
            --name my-cluster
          kubectl get nodes
```

## Commands

**Authentication:**
```bash
azure-login login --client-id <ID> --tenant-id <TENANT> [--subscription-id <SUB>] [--scope <SCOPE>]
```

Parameters:
- `--scope`: OAuth2 scope for the access token (default: `https://management.azure.com/.default`)
  - Azure Resource Manager: `https://management.azure.com/.default` (default)
  - Azure DevOps: `https://app.vssps.visualstudio.com/.default`
  - Microsoft Graph: `https://graph.microsoft.com/.default`
  - Azure Storage: `https://storage.azure.com/.default`
  - Azure Key Vault: `https://vault.azure.net/.default`

**Account Information:**
```bash
azure-login account show
azure-login account get-access-token [--scope <SCOPE>] [--query <JMESPATH>] [-o json|tsv]
```

Parameters:
- `--scope`: Validate that cached token matches this scope (optional). If scope differs, cache is invalidated and you must re-authenticate.

**Azure Container Registry:**
```bash
azure-login acr login --name <REGISTRY>
```

**Azure Kubernetes Service:**
```bash
azure-login aks get-credentials --resource-group <RG> --name <CLUSTER>
```

**Azure Storage:**
```bash
azure-login storage container create --account-name <ACCOUNT> --name <CONTAINER>
azure-login storage container delete --account-name <ACCOUNT> --name <CONTAINER>
azure-login storage queue create --account-name <ACCOUNT> --name <QUEUE>
azure-login storage queue delete --account-name <ACCOUNT> --name <QUEUE>
```

**OIDC Token Management:**
```bash
azure-login oidc get-token [--query <JMESPATH>] [-o json|tsv|table]
```

Use `--help` with any command for detailed usage information.

## Azure Configuration

To use OIDC authentication, configure the following in Azure:

1. **App Registration** - Create an Azure AD App Registration and note the Client ID and Tenant ID

2. **Federated Credentials** - Add a federated credential for your GitHub repository:
   - Entity type: `Environment`, `Branch`, `Pull Request`, or `Tag`
   - Subject identifier example: `repo:org/repo:environment:prod`

3. **RBAC Permissions** - Assign appropriate Azure roles to the service principal

4. **GitHub Actions** - Add workflow permissions:
   ```yaml
   permissions:
     id-token: write
     contents: read
   ```

## Examples

### Package Authentication

**Azure Artifacts (ARM scope - default):**
```bash
# Login without subscription
azure-login login \
  --client-id "12345678-1234-1234-1234-123456789012" \
  --tenant-id "87654321-4321-4321-4321-210987654321" \
  --allow-no-subscriptions

# Get token for Azure Artifacts
TOKEN=$(azure-login account get-access-token --query accessToken -o tsv)
pip install --index-url https://user:${TOKEN}@pkgs.dev.azure.com/org/_packaging/feed/pypi/simple/ package
```

**Azure DevOps Artifacts (DevOps scope):**
```bash
# Login with Azure DevOps scope
azure-login login \
  --client-id "12345678-1234-1234-1234-123456789012" \
  --tenant-id "87654321-4321-4321-4321-210987654321" \
  --allow-no-subscriptions \
  --scope https://app.vssps.visualstudio.com/.default

# Get token for Azure DevOps
TOKEN=$(azure-login account get-access-token --query accessToken -o tsv)

# Use with pip/uv for Azure Artifacts
pip install --index-url https://user:${TOKEN}@pkgs.dev.azure.com/org/_packaging/feed/pypi/simple/ package
```

### AKS Access

```bash
# Login with subscription
azure-login login \
  --client-id "$AZURE_CLIENT_ID" \
  --tenant-id "$AZURE_TENANT_ID" \
  --subscription-id "$AZURE_SUBSCRIPTION_ID"

# Configure kubectl
azure-login aks get-credentials --resource-group prod-rg --name prod-cluster
kubectl get pods
```

### ACR Login

```bash
# Login to Azure
azure-login login \
  --client-id "$AZURE_CLIENT_ID" \
  --tenant-id "$AZURE_TENANT_ID" \
  --subscription-id "$AZURE_SUBSCRIPTION_ID"

# Login to ACR (short name or FQDN)
azure-login acr login --name myregistry
# or
azure-login acr login --name myregistry.azurecr.io

# Now docker/podman commands work
docker pull myregistry.azurecr.io/myimage:latest
docker push myregistry.azurecr.io/myimage:v1.0
```

The identity (service principal / managed identity) must have an ACR role
assignment such as `AcrPull` or `AcrPush` on the target registry.

### Python SDK Authentication

```bash
# Create secure temporary file (mktemp creates with 600 permissions by default)
export AZURE_FEDERATED_TOKEN_FILE=$(mktemp)

# Export environment variables for WorkloadIdentityCredential
export AZURE_CLIENT_ID="12345678-1234-1234-1234-123456789012"
export AZURE_TENANT_ID="87654321-4321-4321-4321-210987654321"

# Write OIDC token to file
azure-login oidc get-token --query value -o tsv > $AZURE_FEDERATED_TOKEN_FILE

# Now Python code using DefaultAzureCredential will work
python your_script.py

# Clean up token file after use
rm -f $AZURE_FEDERATED_TOKEN_FILE
```

**Python code example:**
```python
from azure.identity import DefaultAzureCredential
from azure.storage.blob import BlobServiceClient

# DefaultAzureCredential will automatically use WorkloadIdentityCredential
# when AZURE_FEDERATED_TOKEN_FILE is set
credential = DefaultAzureCredential()
client = BlobServiceClient(account_url="https://myaccount.blob.core.windows.net", credential=credential)
```

## Configuration

### Retry Logic

Automatic retries are **enabled by default** to handle transient network errors common in CI/CD environments.

**Default behavior:**
- 3 attempts (initial + 2 retries)
- 1 second initial delay, exponential backoff (1s, 2s)
- Total worst case: ~18 seconds for OIDC, ~33 seconds for Azure token exchange

**Configuration (optional):**
- `AZURE_LOGIN_RETRY_MAX_ATTEMPTS` - Maximum attempts (default: 3, range: 1-10)
- `AZURE_LOGIN_RETRY_INITIAL_DELAY` - Initial delay in seconds (default: 1, max: 60)
- `AZURE_LOGIN_RETRY_MAX_DELAY` - Maximum delay in seconds (default: 30, max: 300)
- `AZURE_LOGIN_RETRY_BACKOFF_MULTIPLIER` - Backoff multiplier (default: 2.0, max: 5.0)

**Disable retries:**
```yaml
env:
  AZURE_LOGIN_RETRY_MAX_ATTEMPTS: 1  # Single attempt, no retries
```

**Increase retries for unstable networks:**
```yaml
env:
  AZURE_LOGIN_RETRY_MAX_ATTEMPTS: 5
  AZURE_LOGIN_RETRY_INITIAL_DELAY: 2
```

**Retryable errors include:**
- Connection reset by peer
- Connection refused
- Network/host unreachable
- DNS temporary failures
- Timeouts

## Troubleshooting

**"ACTIONS_ID_TOKEN_REQUEST_TOKEN environment variable not set"**
- Not running in GitHub Actions or missing `id-token: write` permission

**"authentication failed: invalid_client"**
- Incorrect client-id or tenant-id
- Federated credentials not configured correctly
- Subject identifier doesn't match workflow

**"not authenticated"**
- Run `azure-login login` first

**"token expired"**
- Run `azure-login login` again to refresh

**"ACR token exchange failed (status 401)"**
- The identity doesn't have an ACR role (AcrPull, AcrPush, etc.) on the registry
- The AAD token may have expired - run `azure-login login` again

**Connection errors in CI**
- Retries are enabled by default (3 attempts)
- Increase retries if needed: `AZURE_LOGIN_RETRY_MAX_ATTEMPTS=5`
- See Configuration section for all retry options

## Development

```bash
git clone https://github.com/cogna-public/azure-login.git
cd azure-login
make build          # Build for current platform
make build-static   # Build all platforms
make test          # Run tests
```

Requires Go 1.25 or later.

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions welcome! Please open an issue or pull request at:
https://github.com/cogna-public/azure-login/issues
