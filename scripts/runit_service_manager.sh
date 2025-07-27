#!/bin/sh
# runit service manager for halsey
#   status – print last failure summary
#   reset  – clear failure state
# default: start halsey, retry on failure, mark failure on exhaustion

set -euo pipefail

# ——— Tunables ————————————————————————————————————————————————
MAX_RETRIES=${MAX_RETRIES:-3}      # attempts before giving up
RETRY_DELAY=${RETRY_DELAY:-2}      # seconds between retries
COOLDOWN=${COOLDOWN:-300}          # back‑off after marked failure
SUMMARY_LINES=${SUMMARY_LINES:-20} # lines kept for `status`
LOG_LIMIT_BYTES=8192               # cap stderr file size (8 KiB)
STATE_DIR=${STATE_DIR:-/var/lib/halsey}
BIN=${BIN:-halsey} #bin on PATH, so just the name
ARGS="run"         # extra args for halsey
# ————————————————————————————————————————————————————————————————

FAILURE_FILE="$STATE_DIR/failure"
LOCK_FILE="$STATE_DIR/failure.lock"
ERR="$STATE_DIR/last-stderr"

mkdir -p "$STATE_DIR"
umask 077
: >"$ERR" 2>/dev/null || true
trap 'rm -f "$ERR"' EXIT

lock() {
	exec 9>"$LOCK_FILE"
	flock -x 9
	: >"$FAILURE_FILE" 2>/dev/null || true
}
unlock() {
	flock -u 9
	exec 9>&-
}

# ——— Sub‑commands ————————————————————————————————————————————
case "${1:-start}" in
status)
	lock
	summary=$(head -n "$SUMMARY_LINES" "$FAILURE_FILE" | tr -d '\n')
	unlock

	if [ -z "$summary" ]; then
		printf '[halsey] No failure state.\n'
	else
		printf '[halsey] Failure: %s\n' "$summary"
	fi
	exit 0
	;;

reset)
	lock
	: >"$FAILURE_FILE"
	unlock
	printf '[halsey] Failure state reset.\n'
	exit 0
	;;
esac
# ———————————————————————————————————————————————————————————————

# Existing failure?
lock
[ -s "$FAILURE_FILE" ] && {
	unlock
	printf '[halsey] In failure state – cooling down %ss…\n' "$COOLDOWN"
	sleep "$COOLDOWN"
	exit 1
}
unlock

# ——— Retry loop ——————————————————————————————————————————————
i=1
while [ "$i" -le "$MAX_RETRIES" ]; do
	printf '[halsey] Run %s of %s…\n' "$i" "$MAX_RETRIES"
	: >"$ERR"

	# run child in background so we can trap & forward signals,
	# but still `exec` inside the subshell to drop the extra PID
	(
		exec "$BIN" $ARGS
	) 2>"$ERR" &
	child=$!

	trap 'kill -TERM "$child" 2>/dev/null' TERM INT HUP
	wait "$child"
	rc=$?
	trap - TERM INT HUP

	if [ "$rc" -eq 0 ]; then
		rm -f "$ERR"
		exit 0
	else
		printf '[halsey] Exit code: %s - retrying…\n' "$rc"
	fi

	# Show full stderr (bounded) then retry
	dd if="$ERR" bs=$LOG_LIMIT_BYTES count=1 2>/dev/null || true
	printf '[halsey] Sleeping %s seconds before retrying…\n' "$RETRY_DELAY"
	sleep "$RETRY_DELAY"
	i=$((i + 1))
done

# ——— Mark failure after exhausting retries ——————————————
printf '[halsey] All %s attempts failed. Marking failure and cooling down %ss.\n' \
	"$MAX_RETRIES" "$COOLDOWN"

lock
{
	printf 'failed at %s:\n' "$(date)"
	head -n "$SUMMARY_LINES" "$ERR"
} >"$FAILURE_FILE"
unlock

sleep "$COOLDOWN"
exit 1
