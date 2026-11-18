#!/bin/sh
# Uploads REVENUECAT_PROJECTS and MCP_TOKEN / MCP_READ_TOKEN (plus optional
# MCP_READ_ONLY) to the Cloudflare Worker.
#
#   production: fnox production profile and projects.json, as Worker secrets.
#   preview:    active fnox profile and projects.sandbox.json, as the Previews base config.
#   dev-json:   prints validated secrets as JSON for .dev.vars.
#
# REVENUECAT_PROJECTS falls back to the fnox profile value when the projects file is absent.
set -eu

self="$(cd "$(dirname "$0")" && pwd)/$(basename "$0")"
cd "$(dirname "$self")/.."

usage() {
	cat >&2 <<-'USAGE'
		Usage: worker-secrets.sh production   upload live secrets to the production worker
		       worker-secrets.sh preview      upload sandbox secrets to the Previews base config
		       worker-secrets.sh dev-json     print the active profile secrets as JSON for .dev.vars
	USAGE
	exit 2
}

# check <projects file> prints validated secrets as a JSON object, or fails without printing secret values.
check() {
	if [ -f "$1" ]; then
		REVENUECAT_PROJECTS="$(cat "$1")"
		export REVENUECAT_PROJECTS
	fi
	jq -n --arg file "$1" '
	  def trimstr: gsub("^\\s+|\\s+$"; "");
	  def safe_label($s): if ($s | startswith("sk_")) then "[REDACTED]" else $s end;
	  ($ENV.REVENUECAT_PROJECTS // "" | trimstr) as $projects_raw
	  | ($ENV.MCP_TOKEN // "" | trimstr) as $full
	  | ($ENV.MCP_READ_TOKEN // "" | trimstr) as $read
	  | ($ENV.MCP_READ_ONLY // "" | trimstr) as $ro
	  | if $projects_raw == "" then
	      error("missing: REVENUECAT_PROJECTS (comes from \($file) or the fnox profile)")
	    else . end
	  | if $full == "" and $read == "" then
	      error("missing: set MCP_TOKEN, MCP_READ_TOKEN, or both")
	    elif $full != "" and $full == $read then
	      error("MCP_READ_TOKEN must differ from MCP_TOKEN")
	    else . end
	  | (try ($projects_raw | fromjson) catch error("REVENUECAT_PROJECTS is not valid JSON")) as $projects
	  | if ($projects | type) != "array" or ($projects | length) == 0 then
	      error("REVENUECAT_PROJECTS must be a non-empty JSON array")
	    else . end
	  | reduce $projects[] as $p ({};
	      if ($p | type) != "object" then
	        error("REVENUECAT_PROJECTS entries must be JSON objects")
	      elif (($p.alias // "") | type) != "string" or (($p.alias // "") | startswith("sk_")) or ((($p.alias // "") | test("^[a-z0-9][a-z0-9_-]*$")) | not) then
	        error("REVENUECAT_PROJECTS: invalid alias \(safe_label(($p.alias // "") | tostring))")
	      elif (($p.project_id // "") | type) != "string" or ((($p.project_id // "") | test("^proj[A-Za-z0-9_-]+$")) | not) then
	        error("REVENUECAT_PROJECTS: invalid project_id for alias \($p.alias)")
	      elif (($p.api_key // "") | type) != "string" or ((($p.api_key // "") | test("^sk_[!-~]+$")) | not) then
	        error("REVENUECAT_PROJECTS: api_key for alias \($p.alias) must start with sk_")
	      elif $p.alias == $p.project_id or has($p.alias) or has($p.project_id) then
	        error("REVENUECAT_PROJECTS: duplicate alias or project_id for alias \($p.alias)")
	      else . + {($p.alias): true, ($p.project_id): true} end
	    )
	  | {REVENUECAT_PROJECTS: ($projects | tojson)}
	  | if $full != "" then .MCP_TOKEN = $full else . end
	  | if $read != "" then .MCP_READ_TOKEN = $read else . end
	  | if $ro == "" then .
	    elif ($ro | ascii_downcase | IN("1", "0", "true", "false", "t", "f")) then .MCP_READ_ONLY = ($ro | ascii_downcase)
	    else error("MCP_READ_ONLY must be a boolean such as 1 or 0") end
	'
}

# Unset first so values from the calling shell cannot stand in for ones the profile lacks.
in_profile() {
	profile="$1"
	shift
	env -u REVENUECAT_PROJECTS -u MCP_TOKEN -u MCP_READ_TOKEN -u MCP_READ_ONLY \
		fnox ${profile:+-P "$profile"} exec -- "$self" "$@"
}

case "${1:-}" in
production) in_profile production upload-from-env production ;;
preview) in_profile "" upload-from-env preview ;;
dev-json) in_profile "" check-from-env ;;
# Internal: run by in_profile inside fnox exec.
check-from-env)
	check projects.sandbox.json
	;;
upload-from-env)
	target="${2:-}"
	case "$target" in
	production) projects_file=projects.json profile=production ;;
	preview) projects_file=projects.sandbox.json profile="${FNOX_PROFILE:-default}" ;;
	*) usage ;;
	esac
	secrets="$(check "$projects_file")"
	worker="$(sed '/^[[:space:]]*\/\//d' wrangler.jsonc | jq -er '.name // empty')"
	echo "worker-secrets: uploading $projects_file from fnox profile $profile -> $target of worker $worker" >&2
	# Secrets reach wrangler only on stdin.
	if [ "$target" = production ]; then
		printf %s "$secrets" | env -u REVENUECAT_PROJECTS -u MCP_TOKEN -u MCP_READ_TOKEN -u MCP_READ_ONLY \
			wrangler secret bulk --name "$worker"
	else
		printf %s "$secrets" | env -u REVENUECAT_PROJECTS -u MCP_TOKEN -u MCP_READ_TOKEN -u MCP_READ_ONLY \
			wrangler preview base-config secret bulk --worker-name "$worker"
	fi
	;;
*) usage ;;
esac
