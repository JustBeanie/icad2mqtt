#!/bin/sh
set -eu

command -v jq >/dev/null 2>&1 || { echo "jq is required" >&2; exit 2; }

assert_value() {
	json=$1
	expression=$2
	want=$3
	got=$(printf '%s\n' "$json" | jq -r "$expression")
	[ "$got" = "$want" ] || { echo "expected $want, got $got" >&2; exit 1; }
}

assert_value '{"publish_raw":true,"ha_discovery":true,"poll_interval":75}' 'if .publish_raw == null then true else .publish_raw end' true
assert_value '{"publish_raw":false,"ha_discovery":false,"poll_interval":0}' 'if .publish_raw == null then true else .publish_raw end' false
assert_value '{"publish_raw":false,"ha_discovery":false,"poll_interval":0}' 'if .ha_discovery == null then false else .ha_discovery end' false
assert_value '{"publish_raw":false,"ha_discovery":false,"poll_interval":0}' 'if .poll_interval == null then 60 else .poll_interval end' 0
assert_value '{}' 'if .publish_raw == null then true else .publish_raw end' true
assert_value '{}' 'if .ha_discovery == null then false else .ha_discovery end' false
assert_value '{}' 'if .poll_interval == null then 60 else .poll_interval end' 60

echo "run.sh jq defaults: pass"
