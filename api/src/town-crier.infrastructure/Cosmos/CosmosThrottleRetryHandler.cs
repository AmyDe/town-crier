using System.Globalization;
using System.Net;
using TownCrier.Infrastructure.Observability;

namespace TownCrier.Infrastructure.Cosmos;

/// <summary>
/// Retries Cosmos DB 429 responses honouring the <c>x-ms-retry-after-ms</c> header
/// with bounded attempts and a total wait budget.
///
/// Cosmos sends a non-standard millisecond-precision retry hint
/// (<c>x-ms-retry-after-ms</c>) which Polly's defaults do not see. The previous
/// pipeline retried 5 times with exponential backoff, leaving the request
/// hanging for &gt;15s before propagating the 429 as a 500. This handler bounds
/// attempts and the per-attempt sleep so user-facing latency stays controlled.
/// </summary>
internal sealed class CosmosThrottleRetryHandler : DelegatingHandler
{
    /// <summary>The Cosmos retry hint header (milliseconds).</summary>
    private const string RetryAfterMsHeader = "x-ms-retry-after-ms";

    private static readonly TimeSpan DefaultFallbackDelay = TimeSpan.FromMilliseconds(100);

    private readonly int maxAttempts;
    private readonly TimeSpan totalWaitBudget;
    private readonly TimeSpan perAttemptCap;
    private readonly Func<int, int> jitter;
    private readonly Func<TimeSpan, CancellationToken, Task> delay;

    public CosmosThrottleRetryHandler(
        int maxAttempts,
        TimeSpan totalWaitBudget,
        TimeSpan perAttemptCap,
        Func<int, int> jitter,
        Func<TimeSpan, CancellationToken, Task> delay)
    {
        ArgumentOutOfRangeException.ThrowIfLessThan(maxAttempts, 1);
        ArgumentNullException.ThrowIfNull(jitter);
        ArgumentNullException.ThrowIfNull(delay);

        this.maxAttempts = maxAttempts;
        this.totalWaitBudget = totalWaitBudget;
        this.perAttemptCap = perAttemptCap;
        this.jitter = jitter;
        this.delay = delay;
    }

    protected override async Task<HttpResponseMessage> SendAsync(
        HttpRequestMessage request,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        HttpResponseMessage? response = null;
        var attempt = 0;
        var totalWaited = TimeSpan.Zero;

        while (true)
        {
            attempt++;

            response = await base.SendAsync(request, cancellationToken).ConfigureAwait(false);

            if (response.StatusCode != HttpStatusCode.TooManyRequests)
            {
                return response;
            }

            // Every 429 the handler observes — including ones that subsequently
            // succeed on retry — increments the throttle counter. This is the
            // canonical RU-pressure signal for dashboards.
            CosmosInstrumentation.Throttles.Add(1);

            if (attempt >= this.maxAttempts)
            {
                return response;
            }

            var wait = this.ComputeWait(response);

            if (totalWaited + wait > this.totalWaitBudget)
            {
                return response;
            }

            response.Dispose();
            await this.delay(wait, cancellationToken).ConfigureAwait(false);
            totalWaited += wait;
        }
    }

    private static TimeSpan? ReadRetryHint(HttpResponseMessage response)
    {
        if (!response.Headers.TryGetValues(RetryAfterMsHeader, out var values))
        {
            return null;
        }

        var raw = values.FirstOrDefault();
        if (string.IsNullOrEmpty(raw))
        {
            return null;
        }

        return double.TryParse(raw, NumberStyles.Float, CultureInfo.InvariantCulture, out var ms)
            ? TimeSpan.FromMilliseconds(ms)
            : null;
    }

    private TimeSpan ComputeWait(HttpResponseMessage response)
    {
        var hint = ReadRetryHint(response) ?? DefaultFallbackDelay;
        var capped = hint > this.perAttemptCap ? this.perAttemptCap : hint;
        var jitterMs = this.jitter(50);
        return capped + TimeSpan.FromMilliseconds(jitterMs);
    }
}
