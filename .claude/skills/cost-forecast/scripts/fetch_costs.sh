#!/usr/bin/env bash
# fetch_costs.sh — wrapper around az rest for the Cost Management query/forecast APIs.
#
# Usage:
#   fetch_costs.sh actuals-by-resource <days>      # Last N days, daily, grouped by ResourceId
#   fetch_costs.sh actuals-by-service  <days>      # Last N days, daily, grouped by ServiceName
#   fetch_costs.sh actuals-total       <days>      # Last N days, daily, ungrouped (subscription total)
#   fetch_costs.sh forecast            <days>      # Forecast for next N days from today
#   fetch_costs.sh resource <resource_id> <days>   # Single resource, daily, last N days
#
# Output: a flat JSON array on stdout. Errors and progress to stderr.
# Pagination via nextLink is handled automatically.

set -euo pipefail

SUB="ae5e40cd-96ef-48d8-950a-2e22cf8f991a"
API="https://management.azure.com/subscriptions/${SUB}/providers/Microsoft.CostManagement"
API_VER="2023-11-01"

cmd="${1:-}"
case "$cmd" in
  actuals-by-resource|actuals-by-service|actuals-total|forecast|resource) ;;
  *) echo "usage: $0 {actuals-by-resource|actuals-by-service|actuals-total|forecast|resource ...} <days>" >&2; exit 2 ;;
esac

today="$(date -u +%Y-%m-%d)"

if [[ "$cmd" == "resource" ]]; then
  resource_id="${2:-}"
  days="${3:-60}"
  [[ -z "$resource_id" ]] && { echo "resource: missing resource_id" >&2; exit 2; }
else
  days="${2:-60}"
fi

# Compute date window (from = today - days for actuals; to = today + days for forecast).
# macOS BSD date and GNU date have different flag syntax — use python for portability.
window() {
  local offset_days="$1"
  python3 -c "from datetime import date, timedelta; print((date.today() + timedelta(days=${offset_days})).isoformat())"
}

case "$cmd" in
  actuals-*|resource)
    from="$(window -"$days")"
    to="$today"
    endpoint="${API}/query?api-version=${API_VER}"
    ;;
  forecast)
    from="$today"
    to="$(window "$days")"
    endpoint="${API}/forecast?api-version=${API_VER}"
    ;;
esac

# Build the request body.
case "$cmd" in
  actuals-by-resource)
    body=$(jq -n --arg from "$from" --arg to "$to" '{
      type:"ActualCost", timeframe:"Custom",
      timePeriod:{from:$from,to:$to},
      dataset:{
        granularity:"Daily",
        aggregation:{totalCost:{name:"Cost",function:"Sum"}},
        grouping:[{type:"Dimension",name:"ResourceId"}]
      }
    }')
    ;;
  actuals-by-service)
    body=$(jq -n --arg from "$from" --arg to "$to" '{
      type:"ActualCost", timeframe:"Custom",
      timePeriod:{from:$from,to:$to},
      dataset:{
        granularity:"Daily",
        aggregation:{totalCost:{name:"Cost",function:"Sum"}},
        grouping:[{type:"Dimension",name:"ServiceName"}]
      }
    }')
    ;;
  actuals-total|forecast)
    body=$(jq -n --arg from "$from" --arg to "$to" '{
      type:"ActualCost", timeframe:"Custom",
      timePeriod:{from:$from,to:$to},
      dataset:{
        granularity:"Daily",
        aggregation:{totalCost:{name:"Cost",function:"Sum"}}
      }
    }')
    ;;
  resource)
    body=$(jq -n --arg from "$from" --arg to "$to" --arg rid "$resource_id" '{
      type:"ActualCost", timeframe:"Custom",
      timePeriod:{from:$from,to:$to},
      dataset:{
        granularity:"Daily",
        aggregation:{totalCost:{name:"Cost",function:"Sum"}},
        filter:{dimensions:{name:"ResourceId",operator:"In",values:[$rid]}}
      }
    }')
    ;;
esac

# Helper: convert Cost Management response (columns + rows) into a flat array of objects.
# Adds an `iso_date` field by parsing UsageDate (YYYYMMDD integer) → YYYY-MM-DD string.
flatten() {
  jq '
    .properties as $p
    | ($p.columns | map(.name)) as $cols
    | $p.rows
    | map(
        . as $r
        | reduce range(0; $cols|length) as $i ({}; .[$cols[$i]] = $r[$i])
        | if .UsageDate then .iso_date = ((.UsageDate|tostring) | "\(.[0:4])-\(.[4:6])-\(.[6:8])") else . end
      )
  '
}

echo "[fetch_costs] $cmd  window=$from..$to" >&2

# First page.
resp=$(az rest --method post --url "$endpoint" --body "$body" 2>/dev/null)
all=$(echo "$resp" | flatten)

# Follow nextLink if present (rare for our queries, but the API does paginate).
next=$(echo "$resp" | jq -r '.properties.nextLink // empty')
while [[ -n "$next" ]]; do
  echo "[fetch_costs] following nextLink" >&2
  resp=$(az rest --method post --url "$next" --body "$body" 2>/dev/null)
  page=$(echo "$resp" | flatten)
  all=$(jq -s 'add' <(echo "$all") <(echo "$page"))
  next=$(echo "$resp" | jq -r '.properties.nextLink // empty')
done

echo "$all"
