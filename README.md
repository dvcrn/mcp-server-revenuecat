# mcp-server-revenuecat-org

Multi-project MCP server for RevenueCat REST API v2, written in Go.

This server addresses two limitations of connecting agents directly to RevenueCat's hosted MCP (`https://mcp.revenuecat.ai/mcp`):

- Interactive OAuth sessions on the hosted MCP expire, breaking unattended and background agents.
- Each RevenueCat API v2 secret key (`sk_...`) is scoped to a single project, so multi-project setups otherwise need a separate MCP server per app or project.

`mcp-server-revenuecat-org` serves all configured RevenueCat projects behind one MCP endpoint (stdio, HTTP, Docker, or Cloudflare Workers), routing each tool call to the right project's `sk_` secret key and supporting strict read-only mode via `MCP_READ_ONLY` or `MCP_READ_TOKEN`.

## Setup

The server reads its configuration from environment variables:

| Variable | Required for | Value |
| --- | --- | --- |
| `REVENUECAT_PROJECTS` | always | JSON array of projects and their RevenueCat API v2 `sk_` secret keys (see dummy example below) |
| `MCP_TOKEN` | HTTP, unless `MCP_READ_TOKEN` is set | Bearer token for every tool (unless `MCP_READ_ONLY` is true), sent as `Authorization: Bearer <token>` or `X-API-Key`. Generate one with `openssl rand -hex 32` |
| `MCP_READ_TOKEN` | HTTP, optional | Bearer token for read-only tools only. Must differ from `MCP_TOKEN` |
| `MCP_READ_ONLY` | optional | `1` or `true` restricts stdio and `MCP_TOKEN` to read-only tools. Non-boolean values fail stdio startup and make every HTTP request return 500 |

`REVENUECAT_PROJECTS` lists each RevenueCat project with a lowercase alias, display name, `proj...` project ID, and project-scoped v2 secret key (`sk_...`). Dummy example data:

```json
[
  { "alias": "ios_app", "name": "Dummy iOS App", "project_id": "proj1ab2c3d4", "api_key": "sk_dummy_projectA" },
  { "alias": "android_app", "name": "Dummy Android App", "project_id": "proj5ef6g7h8", "api_key": "sk_dummy_projectB" }
]
```

Every project-scoped tool accepts `project` as either the configured `alias` (such as `ios_app`) or the allowlisted `project_id` (such as `proj1ab2c3d4`), and rejects any unconfigured project. `list_projects` returns `{alias, name, project_id}` and never exposes `api_key`.

## Read-only mode

Read-only mode can be enforced at three independent layers:

1. Environment flag: setting `MCP_READ_ONLY=1` (or `true`) causes both stdio and HTTP (`MCP_TOKEN`) to register only the 29 read-only tools. Write tools are omitted from `tools/list`, and calling any write tool name fails as an unknown tool.
2. Separate HTTP read token: over HTTP, clients authenticating with `MCP_READ_TOKEN` always receive the read-only server, even when `MCP_TOKEN` has full read-write access. If only `MCP_READ_TOKEN` is configured, the entire HTTP server is read-only.
3. Client guard and RevenueCat key scopes: read-only tools refuse `POST` and `DELETE` in the RevenueCat client before a request is built. For read-only deployments, also issue RevenueCat API v2 keys with read-only project permissions in the RevenueCat dashboard.

## Install

Pick one method and add its entry to `mcpServers` in your client configuration. All `sk_dummy_...` keys and `proj...` IDs below are dummy example values.

### npx, without installing

```json
"revenuecat-org": {
  "command": "npx",
  "args": ["-y", "mcp-server-revenuecat-org"],
  "env": {
    "MCP_READ_ONLY": "1",
    "REVENUECAT_PROJECTS": "[{\"alias\":\"ios_app\",\"name\":\"Dummy iOS App\",\"project_id\":\"proj1ab2c3d4\",\"api_key\":\"sk_dummy_projectA\"}]"
  }
}
```

### npm, mise, or Go

Install with one of:

```sh
npm install -g mcp-server-revenuecat-org
mise use -g go:github.com/dvcrn/mcp-server-revenuecat-org/cmd/mcp@latest
go install github.com/dvcrn/mcp-server-revenuecat-org/cmd/mcp@latest
```

npm installs the binary as `mcp-server-revenuecat-org`. mise and `go install` name it `mcp` after the package directory, so use `mcp` as `command` for those:

```json
"revenuecat-org": {
  "command": "mcp-server-revenuecat-org",
  "env": {
    "REVENUECAT_PROJECTS": "[{\"alias\":\"ios_app\",\"name\":\"Dummy iOS App\",\"project_id\":\"proj1ab2c3d4\",\"api_key\":\"sk_dummy_projectA\"}]"
  }
}
```

### Docker

The image at `ghcr.io/dvcrn/mcp-server-revenuecat-org` serves HTTP on `:8080` by default. For stdio, override the entrypoint:

```json
"revenuecat-org": {
  "command": "docker",
  "args": [
    "run", "-i", "--rm",
    "--entrypoint", "/app/mcp-server-revenuecat-org",
    "-e", "REVENUECAT_PROJECTS", "-e", "MCP_READ_ONLY",
    "ghcr.io/dvcrn/mcp-server-revenuecat-org:latest"
  ],
  "env": {
    "MCP_READ_ONLY": "1",
    "REVENUECAT_PROJECTS": "[{\"alias\":\"ios_app\",\"name\":\"Dummy iOS App\",\"project_id\":\"proj1ab2c3d4\",\"api_key\":\"sk_dummy_projectA\"}]"
  }
}
```

To serve HTTP at `http://localhost:8080/mcp` (with unauthenticated health checks at `GET /health`):

```sh
docker run --rm -p 8080:8080 \
  -e REVENUECAT_PROJECTS -e MCP_TOKEN -e MCP_READ_TOKEN -e MCP_READ_ONLY \
  ghcr.io/dvcrn/mcp-server-revenuecat-org:latest
```

## Run on Cloudflare Workers

Log in with `wrangler login` (or set `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`). Create a gitignored `projects.json` from the dummy template:

```sh
cp projects.example.json projects.json
```

Generate distinct random tokens for `MCP_TOKEN` and (optionally) `MCP_READ_TOKEN`:

```sh
openssl rand -hex 32
```

Upload secrets and deploy:

```sh
wrangler secret put MCP_TOKEN
wrangler secret put MCP_READ_TOKEN   # optional
mise run worker-projects             # uploads projects.json as REVENUECAT_PROJECTS
wrangler deploy
```

`wrangler deploy` builds the Go Wasm bundle through `mise run build-worker`. Update `projects.json` and re-run `mise run worker-projects` to rotate keys or add projects without redeploying code.

Example `.mcp.json` with separate read-write and read-only connectors:

```json
{
  "mcpServers": {
    "revenuecat-org": {
      "type": "http",
      "url": "https://<worker>.<subdomain>.workers.dev/mcp",
      "headers": { "Authorization": "Bearer ${MCP_TOKEN}" }
    },
    "revenuecat-org-readonly": {
      "type": "http",
      "url": "https://<worker>.<subdomain>.workers.dev/mcp",
      "headers": { "Authorization": "Bearer ${MCP_READ_TOKEN}" }
    }
  }
}
```

## Tools

### Read-only tools (29)

Available in all modes (`MCP_TOKEN`, `MCP_READ_TOKEN`, and `MCP_READ_ONLY=1`):

| Tool | Endpoint | Description |
| --- | --- | --- |
| `list_projects` | local config | List configured `{alias, name, project_id}` entries |
| `list_apps` | `GET /v2/projects/{project_id}/apps` | List apps in a project |
| `get_app` | `GET /v2/projects/{project_id}/apps/{app_id}` | Get an app and its store configuration |
| `get_overview_metrics` | `GET /v2/projects/{project_id}/metrics/overview` | Get overview metrics (active subscriptions, trials, MRR, revenue, active/new users) |
| `get_chart_options` | `GET /v2/projects/{project_id}/charts/{chart_name}/options` | Get supported resolutions, segments, filters, and selectors for a chart |
| `get_chart_data` | `GET /v2/projects/{project_id}/charts/{chart_name}` | Query chart time-series data and summary aggregations |
| `list_customers` | `GET /v2/projects/{project_id}/customers` | List or search customers (`search` by email, user ID, or store transaction ID) |
| `get_customer` | `GET /v2/projects/{project_id}/customers/{customer_id}` | Get a customer, active entitlements, and optional expanded `attributes` |
| `list_customer_active_entitlements` | `GET /v2/projects/{project_id}/customers/{customer_id}/active_entitlements` | List active entitlements on a customer |
| `list_customer_aliases` | `GET /v2/projects/{project_id}/customers/{customer_id}/aliases` | List aliases linked to a customer |
| `list_customer_attributes` | `GET /v2/projects/{project_id}/customers/{customer_id}/attributes` | List attributes stored on a customer |
| `list_customer_subscriptions` | `GET /v2/projects/{project_id}/customers/{customer_id}/subscriptions` | List a customer's subscriptions |
| `search_subscriptions` | `GET /v2/projects/{project_id}/subscriptions` | Search subscriptions by `store_subscription_identifier` |
| `get_subscription` | `GET /v2/projects/{project_id}/subscriptions/{subscription_id}` | Get a subscription by ID |
| `list_subscription_entitlements` | `GET /v2/projects/{project_id}/subscriptions/{subscription_id}/entitlements` | List entitlements unlocked by a subscription |
| `list_customer_purchases` | `GET /v2/projects/{project_id}/customers/{customer_id}/purchases` | List a customer's one-time purchases |
| `search_purchases` | `GET /v2/projects/{project_id}/purchases` | Search one-time purchases by `store_purchase_identifier` |
| `get_purchase` | `GET /v2/projects/{project_id}/purchases/{purchase_id}` | Get a one-time purchase by ID |
| `list_purchase_entitlements` | `GET /v2/projects/{project_id}/purchases/{purchase_id}/entitlements` | List entitlements unlocked by a purchase |
| `list_products` | `GET /v2/projects/{project_id}/products` | List products in a project, optionally filtered by `app_id` |
| `get_product` | `GET /v2/projects/{project_id}/products/{product_id}` | Get a product by ID |
| `list_entitlements` | `GET /v2/projects/{project_id}/entitlements` | List entitlements in a project |
| `get_entitlement` | `GET /v2/projects/{project_id}/entitlements/{entitlement_id}` | Get an entitlement by ID |
| `list_entitlement_products` | `GET /v2/projects/{project_id}/entitlements/{entitlement_id}/products` | List products attached to an entitlement |
| `list_offerings` | `GET /v2/projects/{project_id}/offerings` | List offerings in a project |
| `get_offering` | `GET /v2/projects/{project_id}/offerings/{offering_id}` | Get an offering by ID |
| `list_offering_packages` | `GET /v2/projects/{project_id}/offerings/{offering_id}/packages` | List packages in an offering |
| `get_package` | `GET /v2/projects/{project_id}/packages/{package_id}` | Get a package by ID |
| `list_package_products` | `GET /v2/projects/{project_id}/packages/{package_id}/products` | List products and eligibility criteria attached to a package |

### Write tools (26)

Available only when `MCP_READ_ONLY` is unset/`false` and (over HTTP) when authenticated with `MCP_TOKEN`:

| Tool | Endpoint | Description |
| --- | --- | --- |
| `create_customer` | `POST /v2/projects/{project_id}/customers` | Create a customer with `id` and optional initial `attributes` |
| `delete_customer` | `DELETE /v2/projects/{project_id}/customers/{customer_id}` | Permanently delete a customer |
| `set_customer_attributes` | `POST /v2/projects/{project_id}/customers/{customer_id}/attributes` | Set or delete (`delete: true`) customer attributes |
| `grant_customer_entitlement` | `POST /v2/projects/{project_id}/customers/{customer_id}/actions/grant_entitlement` | Grant a promotional entitlement until `expires_at` (Unix ms) |
| `revoke_customer_entitlement` | `POST /v2/projects/{project_id}/customers/{customer_id}/actions/revoke_granted_entitlement` | Revoke an active promotional entitlement grant |
| `assign_customer_offering` | `POST /v2/projects/{project_id}/customers/{customer_id}/actions/assign_offering` | Set (`offering_id`) or clear (`clear: true`) a customer offering override |
| `transfer_customer` | `POST /v2/projects/{project_id}/customers/{customer_id}/actions/transfer` | Transfer subscriptions and purchases to `target_customer_id` |
| `cancel_subscription` | `POST /v2/projects/{project_id}/subscriptions/{subscription_id}/actions/cancel` | Cancel an active Web Billing (`rc_billing`) subscription |
| `refund_subscription` | `POST /v2/projects/{project_id}/subscriptions/{subscription_id}/actions/refund` | Refund the latest Web Billing subscription payment and revoke access |
| `extend_subscription` | `POST /v2/projects/{project_id}/subscriptions/{subscription_id}/actions/extend` | Extend a subscription by `extend_by_days` or `extend_until_ms` |
| `refund_purchase` | `POST /v2/projects/{project_id}/purchases/{purchase_id}/actions/refund` | Refund a Web Billing one-time purchase and revoke access |
| `create_product` | `POST /v2/projects/{project_id}/products` | Create a product in a project |
| `delete_product` | `DELETE /v2/projects/{project_id}/products/{product_id}` | Delete a product from a project |
| `create_entitlement` | `POST /v2/projects/{project_id}/entitlements` | Create an entitlement |
| `update_entitlement` | `POST /v2/projects/{project_id}/entitlements/{entitlement_id}` | Update an entitlement's `display_name` |
| `delete_entitlement` | `DELETE /v2/projects/{project_id}/entitlements/{entitlement_id}` | Delete an entitlement |
| `attach_entitlement_products` | `POST /v2/projects/{project_id}/entitlements/{entitlement_id}/actions/attach_products` | Attach products to an entitlement |
| `detach_entitlement_products` | `POST /v2/projects/{project_id}/entitlements/{entitlement_id}/actions/detach_products` | Detach products from an entitlement |
| `create_offering` | `POST /v2/projects/{project_id}/offerings` | Create an offering |
| `update_offering` | `POST /v2/projects/{project_id}/offerings/{offering_id}` | Update an offering's `display_name`, `is_current`, or `metadata` |
| `delete_offering` | `DELETE /v2/projects/{project_id}/offerings/{offering_id}` | Delete an offering and its packages |
| `create_offering_package` | `POST /v2/projects/{project_id}/offerings/{offering_id}/packages` | Create a package inside an offering |
| `update_package` | `POST /v2/projects/{project_id}/packages/{package_id}` | Update a package's `display_name` or `position` |
| `delete_package` | `DELETE /v2/projects/{project_id}/packages/{package_id}` | Delete a package |
| `attach_package_products` | `POST /v2/projects/{project_id}/packages/{package_id}/actions/attach_products` | Attach products and eligibility criteria to a package |
| `detach_package_products` | `POST /v2/projects/{project_id}/packages/{package_id}/actions/detach_products` | Detach products from a package |

## Direct REST API v2 vs. hosted MCP

RevenueCat's hosted MCP (`https://mcp.revenuecat.ai/mcp`) accepts an `sk_...` secret key as a bearer token, but `mcp-server-revenuecat-org` calls `https://api.revenuecat.com/v2` directly instead of proxying it because:

- Read-only mode executes pure `GET` requests against versioned v2 paths rather than filtering an unversioned upstream tool list where some `readOnlyHint` tools perform `POST` actions.
- Every project-scoped tool accepts configured project aliases or allowlisted `proj...` IDs consistently, whereas hosted MCP tools omit project selectors on single-key endpoints or require raw `proj...` IDs.
- Covers REST v2 endpoints absent from the hosted MCP toolset, including `get_purchase`, `search_subscriptions`, `search_purchases`, customer `active_entitlements`/`aliases`/`attributes` subresources, promotional entitlement revocation, subscription extensions, and Web Billing cancellations/refunds.
