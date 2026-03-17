using System.Diagnostics.CodeAnalysis;
using System.Globalization;
using System.Net;
using System.Runtime.CompilerServices;
using System.Text.Json;
using TownCrier.Application.PlanIt;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Infrastructure.PlanIt;

public sealed class PlanItClient : IPlanItClient
{
    private const int DefaultPageSize = 5000;

    [SuppressMessage("Security", "CA5394:Do not use insecure randomness", Justification = "Jitter for backoff delay does not require cryptographic randomness")]
    private static readonly Random Jitter = new();

    private readonly HttpClient httpClient;
    private readonly PlanItRetryOptions retryOptions;
    private readonly Func<TimeSpan, CancellationToken, Task> delayFunc;

    public PlanItClient(
        HttpClient httpClient,
        PlanItRetryOptions? retryOptions = null,
        Func<TimeSpan, CancellationToken, Task>? delayFunc = null)
    {
        this.httpClient = httpClient;
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
            using var response = await this.SendWithRetryAsync(url, ct).ConfigureAwait(false);
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
        using var response = await this.SendWithRetryAsync(url, ct).ConfigureAwait(false);
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

        return new PlanItSearchResult(applications, planItResponse.Total);
    }

    private static string BuildSearchUrl(string searchText, int authorityId, int page)
    {
        var encodedQuery = Uri.EscapeDataString(searchText);
        return $"/api/applics/json?pg_sz={DefaultPageSize}&sort=-last_different&page={page}&auth={authorityId}&q={encodedQuery}";
    }

    private static string BuildUrl(int authorityId, DateTimeOffset? differentStart, int page)
    {
        var url = $"/api/applics/json?pg_sz={DefaultPageSize}&sort=-last_different&page={page}&auth={authorityId}";

        if (differentStart.HasValue)
        {
            url += $"&different_start={differentStart.Value:yyyy-MM-ddTHH:mm:ss}";
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
            description: record.Description,
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

    private async Task<HttpResponseMessage> SendWithRetryAsync(Uri url, CancellationToken ct)
    {
        for (var attempt = 0; attempt <= this.retryOptions.MaxRetries; attempt++)
        {
            var response = await this.httpClient.GetAsync(url, ct).ConfigureAwait(false);

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

    [SuppressMessage("Security", "CA5394:Do not use insecure randomness", Justification = "Jitter for backoff delay does not require cryptographic randomness")]
    private TimeSpan CalculateBackoffDelay(int attempt)
    {
        var baseMs = this.retryOptions.BaseDelay.TotalMilliseconds;
        var exponentialMs = baseMs * Math.Pow(2, attempt);

        // Add jitter: ±50% of the exponential delay
        var jitterFactor = 0.5 + Jitter.NextDouble();
        var delayMs = exponentialMs * jitterFactor;

        return TimeSpan.FromMilliseconds(delayMs);
    }
}
