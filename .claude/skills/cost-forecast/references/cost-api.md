# Azure Cost Management REST API — payload reference

The installed `az costmanagement` CLI subgroup is incomplete (only `export`). Use `az rest` against the Cost Management REST API directly. All payloads below assume subscription scope.

## Endpoints

| Purpose                       | Endpoint                                                                                                    |
|-------------------------------|-------------------------------------------------------------------------------------------------------------|
| Actuals (historical query)    | `POST /subscriptions/{sub}/providers/Microsoft.CostManagement/query?api-version=2023-11-01`                 |
| Forecast (Azure's projection) | `POST /subscriptions/{sub}/providers/Microsoft.CostManagement/forecast?api-version=2023-11-01`              |
| Budgets                       | `GET  /subscriptions/{sub}/providers/Microsoft.Consumption/budgets?api-version=2023-05-01`                  |

## Timeframe

The query API accepts named timeframes (`MonthToDate`, `BillingMonthToDate`, `TheLastMonth`, `TheLastBillingMonth`, `WeekToDate`, `Custom`). The forecast API in this region only reliably accepts `Custom` with an explicit `timePeriod` block — named timeframes returned `BadRequest` in testing. Always use `Custom` for forecast.

For `Custom`, dates are inclusive and ISO-8601 (`YYYY-MM-DD`). The forecast `to` date can be in the future.

## Body templates

### A. MTD total — daily granularity, ungrouped

Use to compare against Azure's headline number.

```json
{
  "type": "ActualCost",
  "timeframe": "MonthToDate",
  "dataset": {
    "granularity": "Daily",
    "aggregation": { "totalCost": { "name": "Cost", "function": "Sum" } }
  }
}
```

### B. Last 60 days, daily, grouped by ResourceId

Use to build the per-resource timeline (phase 2 of the workflow).

```json
{
  "type": "ActualCost",
  "timeframe": "Custom",
  "timePeriod": { "from": "YYYY-MM-DD", "to": "YYYY-MM-DD" },
  "dataset": {
    "granularity": "Daily",
    "aggregation": { "totalCost": { "name": "Cost", "function": "Sum" } },
    "grouping": [{ "type": "Dimension", "name": "ResourceId" }]
  }
}
```

### C. Last 60 days, daily, grouped by ServiceName

Use for the report's "spend by service" table.

```json
{
  "type": "ActualCost",
  "timeframe": "Custom",
  "timePeriod": { "from": "YYYY-MM-DD", "to": "YYYY-MM-DD" },
  "dataset": {
    "granularity": "Daily",
    "aggregation": { "totalCost": { "name": "Cost", "function": "Sum" } },
    "grouping": [{ "type": "Dimension", "name": "ServiceName" }]
  }
}
```

### D. Forecast over a custom future window

Use for "Azure's projection" in the headline table.

```json
{
  "type": "ActualCost",
  "timeframe": "Custom",
  "timePeriod": { "from": "YYYY-MM-DD", "to": "YYYY-MM-DD" },
  "dataset": {
    "granularity": "Daily",
    "aggregation": { "totalCost": { "name": "Cost", "function": "Sum" } }
  }
}
```

### E. Single resource, daily, last 60 days

Use when you've identified a change-point on one resource and want to confirm before/after.

```json
{
  "type": "ActualCost",
  "timeframe": "Custom",
  "timePeriod": { "from": "YYYY-MM-DD", "to": "YYYY-MM-DD" },
  "dataset": {
    "granularity": "Daily",
    "aggregation": { "totalCost": { "name": "Cost", "function": "Sum" } },
    "filter": {
      "dimensions": {
        "name": "ResourceId",
        "operator": "In",
        "values": ["/subscriptions/.../resourceId"]
      }
    }
  }
}
```

## Response shape

Both query and forecast return:

```json
{
  "properties": {
    "columns": [ { "name": "Cost", "type": "Number" }, { "name": "UsageDate", "type": "Number" }, ... ],
    "rows": [ [ 0.1255, 20260401, "GBP" ], ... ],
    "nextLink": null
  }
}
```

`UsageDate` is an integer in `YYYYMMDD` form. `Cost` is in the subscription's billing currency (GBP for this subscription). When grouping is used, an extra column appears for the grouping dimension; the order matches the `columns` array, so always parse columns first to find indices.

When `nextLink` is non-null, the response is paginated — call the URL in `nextLink` (it's a fully-formed URL with `$skiptoken`). The provided `fetch_costs.sh` script handles pagination for you.

## Rate limits and quotas

Cost Management has a per-subscription rate limit (somewhere around 5-15 requests/min depending on tenant). HTTP 429 is common — back off 30-60 seconds and retry. Don't parallelise queries to the same subscription. Build incrementally so a throttled query doesn't waste already-fetched data.

## Currency

This subscription bills in GBP. The `Currency` column will always be `GBP`. If you ever see `USD` or another currency, something is wrong — stop and investigate before producing the report.
