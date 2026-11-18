# mcp-server-revenuecat

Multi-project MCP server for the RevenueCat REST API v2, written in Go.

RevenueCat's hosted MCP (`https://mcp.revenuecat.ai/mcp`) relies on OAuth sessions whose tokens expire, which breaks unattended and background agents. `mcp-server-revenuecat` uses RevenueCat API v2 secret keys (`sk_...`) that do not expire, and exposes multiple projects behind a single stdio, HTTP, or Cloudflare Worker endpoint.

## Install

### mise

`mise` installs Go packages natively via the `go:` backend (binary name: `mcp-server-revenuecat`):

```sh
mise use -g go:github.com/dvcrn/mcp-server-revenuecat/cmd/mcp-server-revenuecat@latest
```

### npm

Installs the `mcp-server-revenuecat` binary (or run without installing via `npx -y mcp-server-revenuecat`):

```sh
npm install -g mcp-server-revenuecat
```

### go install

Installs the `mcp-server-revenuecat` binary into `$GOPATH/bin`:

```sh
go install github.com/dvcrn/mcp-server-revenuecat/cmd/mcp-server-revenuecat@latest
```

## Setup

Create a gitignored `projects.json` from `projects.example.json`:

```sh
cp projects.example.json projects.json
```

```json
[
  { "alias": "ios_app", "name": "Dummy iOS App", "project_id": "proj1ab2c3d4", "api_key": "sk_dummy_projectA" },
  { "alias": "android_app", "name": "Dummy Android App", "project_id": "proj5ef6g7h8", "api_key": "sk_dummy_projectB" }
]
```

Every project-scoped tool accepts either `alias` or `project_id`. `list_projects` returns `{alias, name, project_id}` without exposing `api_key`.

Environment variables:

| Variable | Required for | Description |
| --- | --- | --- |
| `REVENUECAT_PROJECTS` | always | JSON array of projects (the contents of `projects.json`) |
| `MCP_TOKEN` | HTTP (unless `MCP_READ_TOKEN` is set) | Bearer token (`Authorization: Bearer <token>` or `X-API-Key`). Generate with `openssl rand -hex 32` |
| `MCP_READ_TOKEN` | HTTP, optional | Separate bearer token restricted to read-only tools. Must differ from `MCP_TOKEN` |
| `MCP_READ_ONLY` | optional | `1` or `true` restricts stdio and `MCP_TOKEN` to read-only tools |

Example MCP client config (stdio):

```json
{
  "mcpServers": {
    "revenuecat": {
      "command": "mcp-server-revenuecat",
      "env": {
        "MCP_READ_ONLY": "1",
        "REVENUECAT_PROJECTS": "[{\"alias\":\"ios_app\",\"name\":\"Dummy iOS App\",\"project_id\":\"proj1ab2c3d4\",\"api_key\":\"sk_dummy_projectA\"}]"
      }
    }
  }
}
```

## Run locally

Every install method provides the same `mcp-server-revenuecat` binary. Without installing, use `npx -y mcp-server-revenuecat` in its place, or `go run ./cmd/mcp-server-revenuecat` from a local checkout.

Stdio (default):

```sh
REVENUECAT_PROJECTS="$(cat projects.json)" npx -y mcp-server-revenuecat
```

HTTP (`/mcp` and unauthenticated `GET /health`):

```sh
REVENUECAT_PROJECTS="$(cat projects.json)" MCP_TOKEN="$(openssl rand -hex 32)" npx -y mcp-server-revenuecat -http :8080
```

## Run on Wrangler (Cloudflare Workers)

Log in with `wrangler login` (or set `CLOUDFLARE_API_TOKEN` and `CLOUDFLARE_ACCOUNT_ID`), then upload secrets and deploy:

```sh
wrangler secret put MCP_TOKEN
wrangler secret put MCP_READ_TOKEN   # optional
mise run worker-projects             # validates projects.json and sets REVENUECAT_PROJECTS
wrangler deploy
```

`wrangler deploy` builds the Go Wasm bundle automatically (`bun run build`). For local worker development, run `wrangler dev` (or `mise run dev` if using `fnox` for `.dev.vars`).

Example `.mcp.json` for the deployed worker:

```json
{
  "mcpServers": {
    "revenuecat": {
      "type": "http",
      "url": "https://<worker>.<subdomain>.workers.dev/mcp",
      "headers": { "Authorization": "Bearer ${MCP_TOKEN}" }
    }
  }
}
```

## Tools

### Read-only (29)

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

### Write (26)

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
| `update_offering` | `POST /v2/projects/{project_id}/offerings/{offering_id}` | Update an offering's `display_name`, promote it with `is_current: true`, or set/clear `metadata` |
| `delete_offering` | `DELETE /v2/projects/{project_id}/offerings/{offering_id}` | Delete a non-default offering and its packages |
| `create_offering_package` | `POST /v2/projects/{project_id}/offerings/{offering_id}/packages` | Create a package inside an offering |
| `update_package` | `POST /v2/projects/{project_id}/packages/{package_id}` | Update a package's `display_name` or `position` |
| `delete_package` | `DELETE /v2/projects/{project_id}/packages/{package_id}` | Delete a package |
| `attach_package_products` | `POST /v2/projects/{project_id}/packages/{package_id}/actions/attach_products` | Attach products and eligibility criteria to a package |
| `detach_package_products` | `POST /v2/projects/{project_id}/packages/{package_id}/actions/detach_products` | Detach products from a package |
