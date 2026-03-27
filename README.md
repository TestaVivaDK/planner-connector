# Microsoft Planner MCP Connector

An MCP (Model Context Protocol) server that connects Claude to Microsoft Planner via the Graph API. Manage plans, buckets, tasks, assignments, and checklists through natural conversation.

## Prerequisites

- **Node.js** >= 18
- An **Azure AD App Registration** with the following delegated permissions:
  - `Tasks.ReadWrite`
  - `Group.Read.All`
  - `User.Read`

### Creating an Azure AD App Registration

1. Go to the [Azure Portal](https://portal.azure.com) > **Azure Active Directory** > **App registrations** > **New registration**
2. Name it (e.g. "Planner MCP Connector")
3. Set **Supported account types** to match your org (single tenant is typical)
4. Under **Authentication**, enable **Allow public client flows** (required for device code flow)
5. Under **API permissions**, add the Microsoft Graph delegated permissions listed above
6. Copy the **Application (client) ID** and **Directory (tenant) ID**

## Setup

### Environment Variables

Copy the example and fill in your Azure AD values:

```bash
cp .env.example .env
```

| Variable | Required | Description |
|----------|----------|-------------|
| `PLANNER_MCP_CLIENT_ID` | Yes | Azure AD Application (client) ID |
| `PLANNER_MCP_TENANT_ID` | Yes | Azure AD Directory (tenant) ID |
| `PLANNER_MCP_TOKEN_CACHE_PATH` | No | Custom path for token cache (defaults to `.token-cache.json`) |

### Install as Claude Desktop Extension

Download the latest `.mcpb` from [Releases](../../releases) and install it in Claude Desktop.

Set the environment variables in your Claude Desktop MCP server configuration, or export them before launching.

### Install from Source

```bash
git clone <repo-url>
cd plannner-connector
npm install
cp .env.example .env
# Edit .env with your Azure AD credentials
npm run build
```

## Usage

### Authentication

When you first use any Planner tool, Claude will prompt you to log in:

1. Claude calls `planner-login` which opens your browser automatically
2. A device code is displayed — enter it on the Microsoft sign-in page
3. Complete the Microsoft login and grant permissions
4. The tool confirms success and you're ready to go

Your session token is cached locally so you don't need to log in every time.

### Available Tools

| Tool | Description |
|------|-------------|
| **Auth** | |
| `planner-login` | Authenticate with Microsoft (device code flow) |
| `planner-logout` | Log out and clear cached credentials |
| `planner-auth-status` | Check current authentication status |
| **Plans** | |
| `list-my-plans` | List all Planner plans you're a member of |
| `get-plan` | Get details of a specific plan |
| `create-plan` | Create a new plan |
| `update-plan` | Update plan title or categories |
| `delete-plan` | Delete a plan |
| **Buckets** | |
| `list-buckets` | List buckets in a plan |
| `create-bucket` | Create a new bucket |
| `update-bucket` | Update a bucket |
| `delete-bucket` | Delete a bucket |
| **Tasks** | |
| `list-plan-tasks` | List all tasks in a plan |
| `list-bucket-tasks` | List tasks in a specific bucket |
| `get-task` | Get task details |
| `create-task` | Create a new task |
| `update-task` | Update a task |
| `delete-task` | Delete a task |
| `assign-task` | Assign a user to a task |
| `unassign-task` | Remove a user from a task |
| `move-task` | Move a task to a different bucket |
| **Task Details** | |
| `get-task-details` | Get description and checklist |
| `update-task-details` | Update description, checklist, or references |
| `add-checklist-item` | Add a checklist item |
| `toggle-checklist-item` | Toggle a checklist item's completion |

### Running Locally

**Stdio mode** (default, used by Claude Desktop):
```bash
node dist/index.js
```

**HTTP mode** (for development):
```bash
node dist/index.js --http 3000
# or with a specific host
node dist/index.js --http 127.0.0.1:3000
```

**CLI login** (for testing auth outside of Claude):
```bash
node dist/index.js --login
```

## Development

```bash
npm run dev          # Run with tsx (auto-compiles)
npm run dev:http     # Run HTTP mode with watch
npm run build        # Compile TypeScript
npm run test         # Run tests
npm run lint         # Lint
npm run format       # Format code
```

### Building the `.mcpb` Package

```bash
make package         # Build + package with production deps
make version         # Print current version
make bump-version V=1.2.3  # Set version in manifest + package.json
make clean           # Remove build artifacts
```

### Releasing

Push a version tag to trigger the GitHub Actions release workflow:

```bash
git tag v1.0.0
git push origin v1.0.0
```

This builds the `.mcpb`, creates a GitHub Release, and attaches the package as a download.

## Security

- Tokens are cached locally in `.token-cache.json` (or OS keychain if `keytar` is available)
- The token cache file is created with `0600` permissions (owner read/write only)
- **Never commit `.env` or `.token-cache.json`** — both are in `.gitignore`
- Client ID and tenant ID are read from environment variables, not hardcoded

## License

MIT
