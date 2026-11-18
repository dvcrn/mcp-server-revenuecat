#!/usr/bin/env bash
# Probes which RevenueCat REST API v2 endpoints each configured secret key can
# access without modifying project data.
#
# Usage: probe-permissions.sh [alias_or_proj_id]
#   Reads REVENUECAT_PROJECTS (JSON array) or falls back to REVENUECAT_TEST_API_KEY
#   and REVENUECAT_TEST_PROJECT_ID (for example via `fnox exec -- scripts/probe-permissions.sh`).
set -euo pipefail

readonly API="https://api.revenuecat.com"
filter="${1:-}"

entries_json=""
if [[ -n "${REVENUECAT_PROJECTS:-}" ]]; then
	entries_json="$(printf '%s' "$REVENUECAT_PROJECTS" | jq -ce --arg f "$filter" '
		if type != "array" or length == 0 then
			error("REVENUECAT_PROJECTS must be a non-empty JSON array")
		else
			[ .[] | select($f == "" or .alias == $f or .project_id == $f) ]
		end
	')"
	if [[ "$entries_json" == "[]" ]]; then
		echo "no configured project matching '$filter' in REVENUECAT_PROJECTS" >&2
		exit 2
	fi
elif [[ -n "${REVENUECAT_TEST_API_KEY:-}" ]]; then
	proj="${filter:-${REVENUECAT_TEST_PROJECT_ID:-}}"
	if [[ -z "$proj" ]]; then
		echo "pass a proj... ID or set REVENUECAT_TEST_PROJECT_ID" >&2
		exit 2
	fi
	entries_json="$(jq -cn --arg p "$proj" --arg k "$REVENUECAT_TEST_API_KEY" '[{alias:"test", project_id:$p, api_key:$k}]')"
else
	echo "neither REVENUECAT_PROJECTS nor REVENUECAT_TEST_API_KEY is set; run via 'fnox exec -- $0' or 'mise run probe'" >&2
	exit 2
fi

body_file="$(mktemp)"
trap 'rm -f "$body_file"' EXIT

total_errors=0

probe_project() {
	local alias="$1" project_id="$2" api_key="$3"
	if [[ ! "$project_id" =~ ^proj[A-Za-z0-9_-]+$ ]]; then
		echo "invalid project_id for alias '$alias'" >&2
		exit 2
	fi
	if [[ ! "$api_key" =~ ^sk_[!-~]+$ || "$api_key" == *\"* || "$api_key" == *\\* ]]; then
		echo "invalid api_key for alias '$alias'" >&2
		exit 2
	fi

	local granted=0 denied=0 other=0 errors=0
	echo "=== project: $alias ==="
	printf '%-56s %-7s %-6s %-8s %s\n' ENDPOINT METHOD STATUS VERDICT NOTE

	probe() {
		local kind="$1" method="$2" rel_path="$3"
		local full_path="/v2/projects/${project_id}${rel_path}"
		if [[ "$rel_path" == "/v2/projects" ]]; then
			full_path="/v2/projects"
		fi

		: >"$body_file"
		local status
		# Pass Authorization via curl's stdin config so the secret key never appears in argv.
		if ! status="$(printf 'header = "Authorization: Bearer %s"\n' "$api_key" |
			curl -sS --max-time 10 -o "$body_file" -w '%{http_code}' -X "$method" -K - \
				-H "Accept: application/json" "$API$full_path" 2>/dev/null)"; then
			status="${status:-000}"
			errors=$((errors + 1))
		fi

		local err_type message
		err_type="$(jq -r '.type // empty' "$body_file" 2>/dev/null || true)"
		message="$(jq -r '.message // empty' "$body_file" 2>/dev/null || true)"

		local verdict=other
		if [[ "$status" == 401 || "$status" == 403 || "$err_type" == "authorization_error" || "$err_type" == "authentication_error" ]]; then
			verdict=denied
		else
			case "$kind:$status" in
			read:200 | item:404) verdict=granted ;;
			esac
		fi

		local note=""
		case "$verdict" in
		granted) granted=$((granted + 1)) ;;
		denied)
			denied=$((denied + 1))
			note="${message:-$err_type}"
			note="${note:0:90}"
			;;
		other)
			other=$((other + 1))
			note="${message:-$err_type}"
			note="${note:0:90}"
			;;
		esac

		local display_path="/v2/projects/{project_id}${rel_path%%\?*}"
		if [[ "$rel_path" == "/v2/projects" ]]; then
			display_path="/v2/projects"
		fi
		printf '%-56s %-7s %-6s %-8s %s\n' "$display_path" "$method" "$status" "$verdict" "$note"
	}

	probe read GET "/v2/projects"
	probe read GET "/apps?limit=1"
	probe read GET "/metrics/overview"
	probe read GET "/charts/revenue/options"
	probe read GET "/customers?limit=1"
	probe read GET "/products?limit=1"
	probe read GET "/entitlements?limit=1"
	probe read GET "/offerings?limit=1"

	# Non-mutating write probes against nonexistent IDs: 404 means write scope is granted; 403 means denied.
	probe item DELETE "/customers/cust_probe_nonexistent"
	probe item POST "/subscriptions/sub_probe_nonexistent/actions/cancel"
	probe item POST "/purchases/pur_probe_nonexistent/actions/refund"
	probe item DELETE "/products/prod_probe_nonexistent"
	probe item DELETE "/entitlements/entl_probe_nonexistent"
	probe item DELETE "/offerings/ofrng_probe_nonexistent"
	probe item DELETE "/packages/pkge_probe_nonexistent"

	echo
	echo "project $alias: $granted granted, $denied denied, $other other"
	total_errors=$((total_errors + errors))
}

count="$(printf '%s' "$entries_json" | jq 'length')"
for ((i = 0; i < count; i++)); do
	alias_val="$(printf '%s' "$entries_json" | jq -r ".[$i].alias // \"project_$i\"")"
	proj_val="$(printf '%s' "$entries_json" | jq -r ".[$i].project_id")"
	key_val="$(printf '%s' "$entries_json" | jq -r ".[$i].api_key")"
	probe_project "$alias_val" "$proj_val" "$key_val"
done

if ((total_errors > 0)); then
	echo "$total_errors request(s) failed to reach RevenueCat" >&2
	exit 1
fi
