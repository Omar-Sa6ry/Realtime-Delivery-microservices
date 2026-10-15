#!/bin/sh
# Renders the Alertmanager config so that ALERT_WEBHOOK_URL can be injected.
# Alertmanager itself does not expand environment variables in its config file,
# so the receivers block is materialised here before the binary starts.
set -eu

TEMPLATE="${ALERTMANAGER_TEMPLATE:-/etc/alertmanager/alertmanager.yml.tmpl}"
OUTPUT="${ALERTMANAGER_OUTPUT:-/tmp/alertmanager.yml}"
RECEIVERS="${ALERTMANAGER_RECEIVERS:-/tmp/receivers.yml}"

if [ -n "${ALERT_WEBHOOK_URL:-}" ]; then
    # Both receivers forward: severity="critical" alerts are routed to
    # critical-notifications, everything else (severity="warning"/"info") to
    # default-notifications. Without this the warning rules would be silent.
    cat >"$RECEIVERS" <<EOF
receivers:
  - name: default-notifications
    webhook_configs:
      - url: "${ALERT_WEBHOOK_URL}"
        send_resolved: true
        max_alerts: 0
  - name: critical-notifications
    webhook_configs:
      - url: "${ALERT_WEBHOOK_URL}"
        send_resolved: true
        max_alerts: 0
EOF
    echo "[alertmanager] forwarding all alerts to ${ALERT_WEBHOOK_URL}"
else
    cat >"$RECEIVERS" <<'EOF'
receivers:
  - name: default-notifications
  - name: critical-notifications
EOF
    echo "[alertmanager] ALERT_WEBHOOK_URL not set - alerts stay in the Alertmanager UI only"
fi

awk -v rec="$RECEIVERS" '
    $0 == "__RECEIVERS__" {
        while ((getline line < rec) > 0) print line
        close(rec)
        next
    }
    { print }
' "$TEMPLATE" >"$OUTPUT"

exec /bin/alertmanager --config.file="$OUTPUT" --storage.path=/alertmanager "$@"
