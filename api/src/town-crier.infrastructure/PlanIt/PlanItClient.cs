using System.Globalization;
using System.Net;
using System.Runtime.CompilerServices;
using System.Text.Json;
using TownCrier.Application.PlanIt;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Infrastructure.Observability;

namespace TownCrier.Infrastructure.PlanIt;

public sealed class PlanItClient : IPlanItClient
{
    private const int DefaultPageSize = 100;
    private const int SearchPageSize = 20;

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

    public async IAsyncEnumerable<PlanningApplication> FetchApplicationsAsync(
        int authorityId,
        DateTimeOffset? differentStart,
        [EnumeratorCancellation] CancellationToken ct)
    {
        var page = 1;
        int fetched;

        do
        {
            var url = new Uri(BuildUrl(authorityId, differentStart, page), UriKind.Relative);
            using var response = await this.SendWithThrottleAsync(url, authorityId, ct).ConfigureAwait(false);
            response.EnsureSuccessStatusCode();

            var planItResponse = await JsonSerializer.DeserializeAsync(
                await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false),
                PlanItJsonSerializerContext.Default.PlanItResponse,
                ct).ConfigureAwait(false);

            if (planItResponse is null)
            {
                yield break;
            }

            fetched = planItResponse.Records.Count;

            foreach (var record in planItResponse.Records)
            {
                yield return MapToDomain(record);
            }

            page++;
        }
        while (fetched >= DefaultPageSize);
    }

    public async Task<PlanItSearchResult> SearchApplicationsAsync(
        string searchText,
        int authorityId,
        int page,
        CancellationToken ct)
    {
        var url = new Uri(BuildSearchUrl(searchText, authorityId, page), UriKind.Relative);
        using var response = await this.SendWithThrottleAsync(url, authorityId, ct).ConfigureAwait(false);
        response.EnsureSuccessStatusCode();

        var planItResponse = await JsonSerializer.DeserializeAsync(
            await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false),
            PlanItJsonSerializerContext.Default.PlanItResponse,
            ct).ConfigureAwait(false);

        if (planItResponse is null)
        {
            return new PlanItSearchResult([], 0);
        }

        var applications = planItResponse.Records
            .Select(MapToDomain)
            .ToList();

        return new PlanItSearchResult(applications, planItResponse.Total ?? 0);
    }

    private static string BuildSearchUrl(string searchText, int authorityId, int page)
    {
        var encodedQuery = Uri.EscapeDataString(searchText);
        return $"/api/applics/json?pg_sz={SearchPageSize}&sort=-last_different&page={page}&auth={authorityId}&search={encodedQuery}";
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

    private static bool IsRetryable(HttpStatusCode statusCode)
    {
        return statusCode is
            HttpStatusCode.GatewayTimeout or
            HttpStatusCode.BadGateway or
            HttpStatusCode.ServiceUnavailable or
            HttpStatusCode.RequestTimeout or
            HttpStatusCode.TooManyRequests;
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
