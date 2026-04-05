# PlanIt HTTP Error Metrics Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Surface every non-2xx HTTP response from PlanIt as an OTel counter tagged by status code and authority, with two dedicated Azure dashboard tiles (429s and other errors).

**Architecture:** New `PlanItInstrumentation` static class in infrastructure layer (following `CosmosInstrumentation` pattern). Single instrumentation point in `PlanItClient.SendWithRetryAsync`. Two KQL-based dashboard tiles in the Pulumi-managed Azure dashboard.

**Tech Stack:** System.Diagnostics.Metrics (.NET), Pulumi (C#), TUnit, KQL

**Spec:** `docs/superpowers/specs/2026-04-05-planit-http-error-metrics-design.md`

---

### Task 1: Create PlanItInstrumentation class

**Files:**
- Create: `api/src/town-crier.infrastructure/Observability/PlanItInstrumentation.cs`

- [ ] **Step 1: Create the instrumentation class**

```csharp
using System.Diagnostics.Metrics;

namespace TownCrier.Infrastructure.Observability;

#pragma warning disable SA1202 // Meter must be initialized before public fields that reference it
public static class PlanItInstrumentation
{
    public const string MeterName = "TownCrier.PlanIt";

    private static readonly Meter Meter = new(MeterName);

    public static readonly Counter<long> HttpErrors =
        Meter.CreateCounter<long>(
            "towncrier.planit.http_errors",
            description: "Non-2xx HTTP responses from PlanIt API");
}
#pragma warning restore SA1202
```

- [ ] **Step 2: Verify it compiles**

Run: `dotnet build api/src/town-crier.infrastructure/town-crier.infrastructure.csproj`
Expected: Build succeeded

- [ ] **Step 3: Commit**

```bash
git add api/src/town-crier.infrastructure/Observability/PlanItInstrumentation.cs
git commit -m "feat(api): add PlanItInstrumentation class with HTTP error counter"
```

---

### Task 2: Instrument PlanItClient with HTTP error recording

**Files:**
- Modify: `api/src/town-crier.infrastructure/PlanIt/PlanItClient.cs`

The `SendWithRetryAsync` method (line 150) is the single point where all PlanIt HTTP requests go through. It already inspects status codes for 429 retry logic. We add `authorityId` as a parameter and record every non-2xx response.

- [ ] **Step 1: Add authorityId parameter and metric recording to SendWithRetryAsync**

Change the method signature at line 150 from:

```csharp
    private async Task<HttpResponseMessage> SendWithRetryAsync(Uri url, CancellationToken ct)
```

to:

```csharp
    private async Task<HttpResponseMessage> SendWithRetryAsync(Uri url, int authorityId, CancellationToken ct)
```

Add `using TownCrier.Infrastructure.Observability;` to the top of the file (after line 5, alongside other using statements).

Replace the body of the for-loop (lines 154–177) to record the metric on every non-2xx response. The full updated method:

```csharp
    private async Task<HttpResponseMessage> SendWithRetryAsync(Uri url, int authorityId, CancellationToken ct)
    {
        for (var attempt = 0; attempt <= this.retryOptions.MaxRetries; attempt++)
        {
            if (this.throttleOptions.DelayBetweenRequests > TimeSpan.Zero)
            {
                await this.delayFunc(this.throttleOptions.DelayBetweenRequests, ct).ConfigureAwait(false);
            }

            var response = await this.httpClient.GetAsync(url, ct).ConfigureAwait(false);

            if (!response.IsSuccessStatusCode)
            {
                PlanItInstrumentation.HttpErrors.Add(
                    1,
                    new KeyValuePair<string, object?>("http.response.status_code", (int)response.StatusCode),
                    new KeyValuePair<string, object?>("planit.authority_code", authorityId));
            }

            if (response.StatusCode != (HttpStatusCode)429)
            {
                return response;
            }

            response.Dispose();

            if (attempt == this.retryOptions.MaxRetries)
            {
                throw new HttpRequestException(
                    $"Rate limited by PlanIt API after {this.retryOptions.MaxRetries} retries.",
                    inner: null,
                    HttpStatusCode.TooManyRequests);
            }

            var delay = this.CalculateBackoffDelay(attempt);
            await this.delayFunc(delay, ct).ConfigureAwait(false);
        }

        // Unreachable — loop always returns or throws
        throw new InvalidOperationException();
    }
```

- [ ] **Step 2: Update FetchApplicationsAsync call site**

At line 47, change:

```csharp
            using var response = await this.SendWithRetryAsync(url, ct).ConfigureAwait(false);
```

to:

```csharp
            using var response = await this.SendWithRetryAsync(url, authorityId, ct).ConfigureAwait(false);
```

- [ ] **Step 3: Update SearchApplicationsAsync call site**

At line 79, change:

```csharp
        using var response = await this.SendWithRetryAsync(url, ct).ConfigureAwait(false);
```

to:

```csharp
        using var response = await this.SendWithRetryAsync(url, authorityId, ct).ConfigureAwait(false);
```

- [ ] **Step 4: Verify it compiles**

Run: `dotnet build api/src/town-crier.infrastructure/town-crier.infrastructure.csproj`
Expected: Build succeeded

- [ ] **Step 5: Run existing tests to confirm no regressions**

Run: `dotnet test api/tests/town-crier.infrastructure.tests/ --filter "PlanItClientTests"`
Expected: All 16 existing tests pass (no signature changes visible to callers since `SendWithRetryAsync` is private)

- [ ] **Step 6: Commit**

```bash
git add api/src/town-crier.infrastructure/PlanIt/PlanItClient.cs
git commit -m "feat(api): record PlanIt HTTP errors in SendWithRetryAsync"
```

---

### Task 3: Add tests for HTTP error metric recording

**Files:**
- Modify: `api/tests/town-crier.infrastructure.tests/PlanIt/FakePlanItHandler.cs`
- Modify: `api/tests/town-crier.infrastructure.tests/PlanIt/PlanItClientTests.cs`

We need to verify the counter fires with correct tags. `System.Diagnostics.Metrics` provides `MeterListener` for in-process metric collection in tests.

First, add `using System.Diagnostics.Metrics;` to `PlanItClientTests.cs` (for `MeterListener`). The file already has `using System.Net;` for `HttpStatusCode`.

- [ ] **Step 1: Add SetupStatusCodeResponse to FakePlanItHandler**

Add a new dictionary and method to `FakePlanItHandler` to return arbitrary status codes. Add this field after line 11:

```csharp
    private readonly Dictionary<string, HttpStatusCode> statusCodeResponses = new();
```

Add this method after the `SetupRateLimitForever` method (after line 29):

```csharp
    public void SetupStatusCodeResponse(string urlContains, HttpStatusCode statusCode)
    {
        this.statusCodeResponses[urlContains] = statusCode;
    }
```

In `SendAsync`, add a check for status code responses before the existing response lookup. Insert after the rate limit block (after line 45), before the response lookup (line 47):

```csharp
        foreach (var (key, statusCode) in this.statusCodeResponses)
        {
            if (url.Contains(key, StringComparison.Ordinal))
            {
                return Task.FromResult(new HttpResponseMessage(statusCode));
            }
        }
```

- [ ] **Step 2: Write test for 429 metric recording**

Add to `PlanItClientTests.cs`:

```csharp
    [Test]
    public async Task Should_RecordHttpErrorMetric_When_ApiReturns429()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupRateLimitThenSuccess("page=1", count: 2, SingleRecordResponse);
        var client = CreateClient(handler, retryOptions: new PlanItRetryOptions { MaxRetries = 3, BaseDelay = TimeSpan.FromMilliseconds(1) });

        var recorded = new List<(long Value, int StatusCode, int AuthorityCode)>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.planit.http_errors")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            var statusCode = 0;
            var authorityCode = 0;
            foreach (var tag in tags)
            {
                if (tag.Key == "http.response.status_code")
                {
                    statusCode = (int)tag.Value!;
                }

                if (tag.Key == "planit.authority_code")
                {
                    authorityCode = (int)tag.Value!;
                }
            }

            recorded.Add((measurement, statusCode, authorityCode));
        });
        listener.Start();

        // Act
        await ConsumeAsync(client, differentStart: null, authorityId: 292);

        // Assert — 2 rate limit responses recorded
        await Assert.That(recorded).HasCount().EqualTo(2);
        await Assert.That(recorded[0].StatusCode).IsEqualTo(429);
        await Assert.That(recorded[0].AuthorityCode).IsEqualTo(292);
        await Assert.That(recorded[1].StatusCode).IsEqualTo(429);
    }
```

- [ ] **Step 3: Run test to verify it passes**

Run: `dotnet test api/tests/town-crier.infrastructure.tests/ --filter "Should_RecordHttpErrorMetric_When_ApiReturns429"`
Expected: PASS

- [ ] **Step 4: Write test for non-429 error metric recording**

Add to `PlanItClientTests.cs`:

```csharp
    [Test]
    public async Task Should_RecordHttpErrorMetric_When_ApiReturns500()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupStatusCodeResponse("page=1", HttpStatusCode.InternalServerError);
        var client = CreateClient(handler);

        var recorded = new List<(long Value, int StatusCode, int AuthorityCode)>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.planit.http_errors")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            var statusCode = 0;
            var authorityCode = 0;
            foreach (var tag in tags)
            {
                if (tag.Key == "http.response.status_code")
                {
                    statusCode = (int)tag.Value!;
                }

                if (tag.Key == "planit.authority_code")
                {
                    authorityCode = (int)tag.Value!;
                }
            }

            recorded.Add((measurement, statusCode, authorityCode));
        });
        listener.Start();

        // Act & Assert — EnsureSuccessStatusCode throws, but metric should still be recorded
        await Assert.ThrowsAsync<HttpRequestException>(
            async () => await ConsumeAsync(client, differentStart: null, authorityId: 314));

        await Assert.That(recorded).HasCount().EqualTo(1);
        await Assert.That(recorded[0].StatusCode).IsEqualTo(500);
        await Assert.That(recorded[0].AuthorityCode).IsEqualTo(314);
    }
```

- [ ] **Step 5: Write test for success (no metric recorded)**

Add to `PlanItClientTests.cs`:

```csharp
    [Test]
    public async Task Should_NotRecordHttpErrorMetric_When_ApiReturns200()
    {
        // Arrange
        using var handler = new FakePlanItHandler();
        handler.SetupJsonResponse("page=1", SingleRecordResponse);
        var client = CreateClient(handler);

        var recorded = new List<long>();
        using var listener = new MeterListener();
        listener.InstrumentPublished = (instrument, listener) =>
        {
            if (instrument.Name == "towncrier.planit.http_errors")
            {
                listener.EnableMeasurementEvents(instrument);
            }
        };
        listener.SetMeasurementEventCallback<long>((instrument, measurement, tags, _) =>
        {
            recorded.Add(measurement);
        });
        listener.Start();

        // Act
        await ConsumeAsync(client, differentStart: null);

        // Assert — no errors recorded for successful response
        await Assert.That(recorded).HasCount().EqualTo(0);
    }
```

- [ ] **Step 6: Run all metric tests**

Run: `dotnet test api/tests/town-crier.infrastructure.tests/ --filter "Should_RecordHttpErrorMetric|Should_NotRecordHttpErrorMetric"`
Expected: All 3 new tests pass

- [ ] **Step 7: Run full PlanItClient test suite for regressions**

Run: `dotnet test api/tests/town-crier.infrastructure.tests/ --filter "PlanItClientTests"`
Expected: All tests pass (existing + 3 new)

- [ ] **Step 8: Commit**

```bash
git add api/tests/town-crier.infrastructure.tests/PlanIt/FakePlanItHandler.cs api/tests/town-crier.infrastructure.tests/PlanIt/PlanItClientTests.cs
git commit -m "test(api): add tests for PlanIt HTTP error metric recording"
```

---

### Task 4: Register PlanIt meter in OTel configuration

**Files:**
- Modify: `api/src/town-crier.worker/Program.cs:44-45`
- Modify: `api/src/town-crier.web/Program.cs:36-37`

- [ ] **Step 1: Add using statement and meter registration in worker**

In `api/src/town-crier.worker/Program.cs`, add `using TownCrier.Infrastructure.Observability;` alongside existing using statements at the top of the file (if not already present — check first).

At line 45, after `.AddMeter(CosmosInstrumentation.MeterName)`, add:

```csharp
            .AddMeter(PlanItInstrumentation.MeterName)
```

- [ ] **Step 2: Add meter registration in web**

In `api/src/town-crier.web/Program.cs`, add `using TownCrier.Infrastructure.Observability;` alongside existing using statements at the top of the file (if not already present — check first).

At line 37, after `.AddMeter(CosmosInstrumentation.MeterName)`, add:

```csharp
            .AddMeter(PlanItInstrumentation.MeterName)
```

- [ ] **Step 3: Verify both projects compile**

Run: `dotnet build api/src/town-crier.worker/town-crier.worker.csproj && dotnet build api/src/town-crier.web/town-crier.web.csproj`
Expected: Both build succeeded

- [ ] **Step 4: Commit**

```bash
git add api/src/town-crier.worker/Program.cs api/src/town-crier.web/Program.cs
git commit -m "feat(api): register PlanIt meter in worker and web OTel config"
```

---

### Task 5: Add dashboard tiles for PlanIt errors

**Files:**
- Modify: `infra/SharedStack.cs:268-272`

Two KQL tiles in a new row 4 (Y=12). Using `KqlTile` because the `MetricTile` helper doesn't support dimension filters.

- [ ] **Step 1: Add the two new dashboard tiles**

In `infra/SharedStack.cs`, after the last tile in the Parts array (the "API Errors" tile ending around line 272), add:

```csharp
                            // Row 4: PlanIt API Health
                            new DashboardPartsArgs
                            {
                                Position = new DashboardPartsPositionArgs { X = 0, Y = 12, ColSpan = 6, RowSpan = 4 },
                                Metadata = KqlTile(
                                    appInsights.Id,
                                    "customMetrics | where name == 'towncrier.planit.http_errors' | extend status = tostring(customDimensions['http.response.status_code']) | where status == '429' | summarize Value=sum(value) by timestamp=bin(timestamp, 1h) | render timechart",
                                    "PlanIt 429s"),
                            },
                            new DashboardPartsArgs
                            {
                                Position = new DashboardPartsPositionArgs { X = 6, Y = 12, ColSpan = 6, RowSpan = 4 },
                                Metadata = KqlTile(
                                    appInsights.Id,
                                    "customMetrics | where name == 'towncrier.planit.http_errors' | extend status = tostring(customDimensions['http.response.status_code']) | where status != '429' | summarize Value=sum(value) by timestamp=bin(timestamp, 1h), status | render timechart",
                                    "PlanIt Errors"),
                            },
```

- [ ] **Step 2: Verify infra project compiles**

Run: `dotnet build infra/infra.csproj`
Expected: Build succeeded

- [ ] **Step 3: Commit**

```bash
git add infra/SharedStack.cs
git commit -m "feat(infra): add PlanIt 429s and PlanIt Errors dashboard tiles"
```

---

### Task 6: Final verification

- [ ] **Step 1: Run full test suite**

Run: `dotnet test api/`
Expected: All tests pass

- [ ] **Step 2: Run full solution build**

Run: `dotnet build api/ && dotnet build infra/`
Expected: Both build successfully

- [ ] **Step 3: Verify formatting**

Run: `dotnet format api/ --verify-no-changes`
Expected: No formatting issues
