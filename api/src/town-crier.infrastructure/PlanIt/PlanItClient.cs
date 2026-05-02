using System.Globalization;
using System.Net;
using System.Text.Json;
using TownCrier.Application.PlanIt;
using TownCrier.Application.Polling;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Infrastructure.Observability;

namespace TownCrier.Infrastructure.PlanIt;

public sealed class PlanItClient : IPlanItClient
{
    private const int DefaultPageSize = 100;

    private readonly HttpClient httpClient;
    private readonly PlanItThrottleOptions throttleOptions;
    private readonly PlanItRetryOptions retryOptions;
    private readonly Func<TimeSpan, CancellationToken, Task> delayFunc;

    public PlanItClient(
        HttpClient httpClient,
        PlanItThrottleOptions? throttleOptions = null,
        PlanItRetryOptions? retryOptions = null,
        Func<TimeSpan, CancellationToken, Task>? delayFunc = null)
    {
        this.httpClient = httpClient;
        this.throttleOptions = throttleOptions ?? new PlanItThrottleOptions();
        this.retryOptions = retryOptions ?? new PlanItRetryOptions();
        this.delayFunc = delayFunc ?? Task.Delay;
    }

    public async Task<FetchPageResult> FetchApplicationsPageAsync(
        int authorityId,
        DateTimeOffset? differentStart,
        int page,
        CancellationToken ct)
    {
        var url = new Uri(BuildUrl(authorityId, differentStart, page), UriKind.Relative);
        using var response = await this.SendWithThrottleAsync(url, authorityId, ct).ConfigureAwait(false);
        EnsureSuccessOrThrow(response);

        var planItResponse = await JsonSerializer.DeserializeAsync(
            await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false),
            PlanItJsonSerializerContext.Default.PlanItResponse,
            ct).ConfigureAwait(false);

        if (planItResponse is null)
        {
            return new FetchPageResult(page, [], Total: null, HasMorePages: false);
        }

        var applications = planItResponse.Records
            .Select(MapToDomain)
            .ToList();

        // Page-fill heuristic: a full page means more records may follow. This mirrors
        // the previous client-side pagination loop's exit condition.
        var hasMorePages = applications.Count >= DefaultPageSize;

        return new FetchPageResult(page, applications, planItResponse.Total, hasMorePages);
    }

    public async Task<PlanningApplication?> GetByUidAsync(string uid, CancellationToken ct)
    {
        // PlanIt's per-application endpoint returns a single record (not wrapped
        // in {"records":[]}). 404 indicates the uid is unknown — surface that as
        // null so callers can decide the application doesn't exist.
        var url = new Uri(BuildByUidUrl(uid), UriKind.Relative);

        // authorityId is unknown for a uid-only lookup; pass 0 for telemetry tagging.
        using var response = await this.SendWithThrottleAsync(url, authorityId: 0, ct).ConfigureAwait(false);

        if (response.StatusCode == HttpStatusCode.NotFound)
        {
            return null;
        }

        EnsureSuccessOrThrow(response);

        var record = await JsonSerializer.DeserializeAsync(
            await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false),
            PlanItJsonSerializerContext.Default.PlanItApplicationRecord,
            ct).ConfigureAwait(false);

        return record is null ? null : MapToDomain(record);
    }

    private static string BuildByUidUrl(string uid)
    {
        // Don't escape '/' — PlanIt uids contain slashes (e.g. "26/01471/TR")
        // and the routing depends on the literal path segments.
        return $"/planapplic/{uid}/json";
    }

    private static string BuildUrl(int authorityId, DateTimeOffset? differentStart, int page)
    {
        var url = $"/api/applics/json?pg_sz={DefaultPageSize}&sort=last_different&page={page}&auth={authorityId}";

        if (differentStart.HasValue)
        {
            url += $"&different_start={differentStart.Value:yyyy-MM-dd}";
        }

        return url;
    }

    private static PlanningApplication MapToDomain(PlanItApplicationRecord record)
    {
        return new PlanningApplication(
            name: record.Name,
            uid: record.Uid,
            areaName: record.AreaName,
            areaId: record.AreaId,
            address: record.Address,
            postcode: record.Postcode,
            description: record.Description ?? string.Empty,
            appType: record.AppType,
            appState: record.AppState,
            appSize: record.AppSize,
            startDate: ParseDate(record.StartDate),
            decidedDate: ParseDate(record.DecidedDate),
            consultedDate: ParseDate(record.ConsultedDate),
            longitude: record.LocationX,
            latitude: record.LocationY,
            url: record.Url,
            link: record.Link,
            lastDifferent: DateTimeOffset.Parse(record.LastDifferent, CultureInfo.InvariantCulture));
    }

    private static DateOnly? ParseDate(string? value)
    {
        if (string.IsNullOrEmpty(value))
        {
            return null;
        }

        return DateOnly.Parse(value, CultureInfo.InvariantCulture);
    }

    private static void EnsureSuccessOrThrow(HttpResponseMessage response)
    {
        if (response.IsSuccessStatusCode)
        {
            return;
        }

        if (response.StatusCode == HttpStatusCode.TooManyRequests)
        {
            var header = response.Headers.RetryAfter;
            string? headerValue = null;
            if (header?.Delta is { } delta)
            {
                headerValue = ((int)delta.TotalSeconds).ToString(CultureInfo.InvariantCulture);
            }
            else if (header?.Date is { } date)
            {
                headerValue = date.UtcDateTime.ToString("R", CultureInfo.InvariantCulture);
            }

            var retryAfter = RetryAfterParser.Parse(headerValue, DateTimeOffset.UtcNow);
            throw new PlanItRateLimitException(retryAfter);
        }

        response.EnsureSuccessStatusCode();
    }

    private static bool IsRetryable(HttpStatusCode statusCode)
    {
        // 429 is intentionally NOT retried here — it throws PlanItRateLimitException
        // immediately via EnsureSuccessOrThrow so the scheduler can use the
        // Retry-After header to choose the next run time. Internal retries on 429
        // burn handler wall-clock budget and risk Service Bus lock expiry.
        return statusCode is
            HttpStatusCode.GatewayTimeout or
            HttpStatusCode.BadGateway or
            HttpStatusCode.ServiceUnavailable or
            HttpStatusCode.RequestTimeout;
    }

    private async Task<HttpResponseMessage> SendWithThrottleAsync(Uri url, int authorityId, CancellationToken ct)
    {
        var maxAttempts = 1 + this.retryOptions.MaxRetries;

        for (var attempt = 0; attempt < maxAttempts; attempt++)
        {
            if (this.throttleOptions.DelayBetweenRequests > TimeSpan.Zero)
            {
                await this.delayFunc(this.throttleOptions.DelayBetweenRequests, ct).ConfigureAwait(false);
            }

            var response = await this.httpClient.GetAsync(url, ct).ConfigureAwait(false);

            if (response.IsSuccessStatusCode)
            {
                return response;
            }

            PlanItInstrumentation.HttpErrors.Add(
                1,
                new KeyValuePair<string, object?>("http.response.status_code", (int)response.StatusCode),
                new KeyValuePair<string, object?>("planit.authority_code", authorityId));

            var isLastAttempt = attempt >= maxAttempts - 1;

            if (!IsRetryable(response.StatusCode) || isLastAttempt)
            {
                return response;
            }

            var backoff = this.ComputeBackoff(response.StatusCode, attempt);
            PlanItInstrumentation.Retries.Add(
                1,
                new KeyValuePair<string, object?>("http.response.status_code", (int)response.StatusCode),
                new KeyValuePair<string, object?>("planit.authority_code", authorityId));

            response.Dispose();
            await this.delayFunc(backoff, ct).ConfigureAwait(false);
        }

        // Unreachable -- the loop always returns
        throw new InvalidOperationException("Retry loop exited unexpectedly.");
    }

    private TimeSpan ComputeBackoff(HttpStatusCode statusCode, int attempt)
    {
        if (statusCode == HttpStatusCode.TooManyRequests)
        {
            // Exponential backoff starting from rate limit backoff
            return this.retryOptions.RateLimitBackoff * Math.Pow(2, attempt);
        }

        // Exponential backoff starting from initial backoff: 1s, 2s, 4s, ...
        return this.retryOptions.InitialBackoff * Math.Pow(2, attempt);
    }
}
