#!/bin/sh
set -eu
base="${DASHBOARD_URL:-http://localhost:8080}"
at="2026-09-29T10:00:00Z"
for stage in asset_ingested job_processed creator_delivered; do
  curl --fail-with-body -X POST "$base/events" -H 'Content-Type: application/json' \
    -d "{\"asset_id\":\"asset-7\",\"stage\":\"$stage\",\"duration_ms\":840,\"at\":\"$at\"}"
done
