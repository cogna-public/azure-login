# azure-login

A lightweight, statically-linked Go CLI tool for Azure authentication in CI/CD environments. Designed as a drop-in replacement for Azure CLI authentication commands in GitHub Actions workflows.

## Features

- **Statically linked** - Single binary with no external runtime dependencies
- **Fast** - Significantly faster than Python-based Azure CLI
- **Lightweight** - ~10MB binary vs 1GB+ Azure CLI installation
- **OIDC authentication** - Native support for GitHub Actions workload identity
- **Compatible** - Drop-in replacement for Azure CLI auth commands

## Supported Commands

### Authentication
```bash
azure-login login --client-id <ID> --tenant-id <TENANT> [--subscription-id <SUB>] [--allow-no-subscriptions]
```

### Account Information
```bash
azure-login account show
azure-login account get-access-token [--query <JMESPATH>] [-o json|tsv]
```

## Installation

### From GitHub Releases

Download the latest release for your platform:

```bash
# Linux AMD64
curl -L https://github.com/cogna-public/azure-login/releases/latest/download/azure-login-linux-amd64 -o azure-login
chmod +x azure-login
sudo mv azure-login /usr/local/bin/

# macOS AMD64
curl -L https://github.com/cogna-public/azure-login/releases/latest/download/azure-login-darwin-amd64 -o azure-login
chmod +x azure-login
sudo mv azure-login /usr/local/bin/

# macOS ARM64 (Apple Silicon)
curl -L https://github.com/cogna-public/azure-login/releases/latest/download/azure-login-darwin-arm64 -o azure-login
chmod +x azure-login
sudo mv azure-login /usr/local/bin/
```

### Build from Source

```bash
git clone https://github.com/cogna-public/azure-login.git
cd azure-login
make build
make install
```

## Usage

### In GitHub Actions

#### Using Direct Commands

```yaml
- name: Azure Login
  run: |
    azure-login login \
      --client-id ${{ vars.AZURE_CLIENT_ID }} \
      --tenant-id ${{ vars.AZURE_TENANT_ID }} \
      --subscription-id ${{ vars.AZURE_SUBSCRIPTION_ID }}

- name: Get Access Token
  id: get_token
  run: |
    echo "token=$(azure-login account get-access-token --query accessToken -o tsv)" >> $GITHUB_OUTPUT

- name: Use Token
  env:
    AZURE_TOKEN: ${{ steps.get_token.outputs.token }}
  run: |
    # Use token for authentication
    echo "Token expires: $(azure-login account get-access-token --query expiresOn)"
```

#### As Azure CLI Alias

Add this to your workflow to use `azure-login` as a drop-in replacement:

```yaml
- name: Setup Azure CLI Alias
  run: |
    echo 'alias az="azure-login"' >> ~/.bashrc
    source ~/.bashrc

- name: Login
  run: az login --client-id ${{ vars.AZURE_CLIENT_ID }} --tenant-id ${{ vars.AZURE_TENANT_ID }}

- name: Get Token
  run: az account get-access-token --query accessToken -o tsv
```

### Environment Variables

The following environment variables are supported:

| Variable | Description |
|----------|-------------|
| `AZURE_CLIENT_ID` | Override `--client-id` parameter |
| `AZURE_TENANT_ID` | Override `--tenant-id` parameter |
| `AZURE_SUBSCRIPTION_ID` | Override `--subscription-id` parameter |
| `AZURE_CONFIG_DIR` | Override config directory (default: `~/.azure`) |
| `ACTIONS_ID_TOKEN_REQUEST_TOKEN` | GitHub Actions OIDC token (auto-set) |
| `ACTIONS_ID_TOKEN_REQUEST_URL` | GitHub Actions OIDC URL (auto-set) |

### Authentication Flow

1. **GitHub Actions** generates an OIDC token
2. **azure-login** reads the token from environment variables
3. **Azure AD** exchanges the OIDC token for an Azure access token
4. **Token cached** in `~/.azure/azure-login-token.json` (0600 permissions)
5. **Subsequent commands** use the cached token automatically

### Examples

#### Get Access Token for Package Authentication

```bash
# Login
azure-login login \
  --client-id "12345678-1234-1234-1234-123456789012" \
  --tenant-id "87654321-4321-4321-4321-210987654321" \
  --allow-no-subscriptions

# Get token
TOKEN=$(azure-login account get-access-token --query accessToken -o tsv)

# Use with pip
pip install --index-url https://user:${TOKEN}@pkgs.dev.azure.com/org/_packaging/feed/pypi/simple/ my-package
```

#### Check Token Expiration

```bash
azure-login account get-access-token --query expiresOn
# Output: "2024-10-16 14:30:00.000000"
```

#### View Account Information

```bash
azure-login account show
```

Output:
```json
{
  "environmentName": "AzureCloud",
  "id": "12345678-1234-1234-1234-123456789012",
  "name": "Azure Subscription",
  "tenantId": "87654321-4321-4321-4321-210987654321",
  "user": {
    "name": "12345678-1234-1234-1234-123456789012",
    "type": "servicePrincipal"
  }
}
```

## Azure Configuration Requirements

For OIDC authentication to work, you need:

1. **Azure AD Application Registration**
   - Create an App Registration in Azure AD
   - Note the Client ID and Tenant ID

2. **Federated Credentials**
   - Add federated credential for your GitHub repository
   - Entity type: `Environment`, `Branch`, `Pull Request`, or `Tag`
   - Subject identifier: `repo:org/repo:environment:prod` (example)

3. **RBAC Role Assignments**
   - Assign necessary roles to the service principal
   - Example: `Reader` for subscription access

4. **GitHub Actions Permissions**
   - Add `id-token: write` to workflow permissions:
   ```yaml
   permissions:
     id-token: write
     contents: read
   ```

## Command Reference

### login

Authenticate to Azure using OIDC workload identity.

```bash
azure-login login [flags]
```

**Flags:**
- `--client-id` (required) - Azure Application (Client) ID
- `--tenant-id` (required) - Azure Active Directory Tenant ID
- `--subscription-id` (optional) - Azure Subscription ID
- `--allow-no-subscriptions` (optional) - Allow authentication without subscription

### account show

Display current account information.

```bash
azure-login account show [-o json|tsv]
```

**Flags:**
- `-o, --output` - Output format: json, tsv, table (default: json)

### account get-access-token

Get an Azure access token for resource authentication.

```bash
azure-login account get-access-token [--query <JMESPATH>] [-o json|tsv]
```

**Flags:**
- `--query` - JMESPath query string to filter output
- `-o, --output` - Output format: json, tsv, table (default: json)

**Examples:**
```bash
# Get full token information
azure-login account get-access-token

# Get only the token value
azure-login account get-access-token --query accessToken -o tsv

# Get expiration time
azure-login account get-access-token --query expiresOn
```

## Development

### Prerequisites

- Go 1.25 or later
- Make

### Building

```bash
# Build for current platform
make build

# Build statically-linked binaries for all platforms
make build-static

# Run tests
make test

# Format code
make fmt
```

## Roadmap

### v1.0 (Current)
- [x] OIDC authentication
- [x] Token caching
- [x] `login` command
- [x] `account show` command
- [x] `account get-access-token` command
- [x] JMESPath query support
- [x] JSON/TSV output formats

### v2.0 (Future)
- [ ] `aks get-credentials` command
- [ ] Kubeconfig integration
- [ ] Token refresh logic
- [ ] Multiple token scopes

## Troubleshooting

### "ACTIONS_ID_TOKEN_REQUEST_TOKEN environment variable not set"

**Cause:** Not running in GitHub Actions or missing permissions.

**Solution:** Add to workflow:
```yaml
permissions:
  id-token: write
```

### "authentication failed: invalid_client"

**Cause:** Incorrect client-id or tenant-id, or federated credentials not configured.

**Solution:**
1. Verify client-id and tenant-id are correct
2. Check federated credentials in Azure AD
3. Verify subject identifier matches your workflow

### "not authenticated"

**Cause:** No cached token found.

**Solution:** Run `azure-login login` first.

### "token expired"

**Cause:** Cached token has expired.

**Solution:** Run `azure-login login` again to refresh.

## License

MIT License - see LICENSE file for details.

## Contributing

Contributions are welcome! Please open an issue or pull request.

## Support

For issues, questions, or feature requests, please open an issue on GitHub:
https://github.com/cogna-public/azure-login/issues
