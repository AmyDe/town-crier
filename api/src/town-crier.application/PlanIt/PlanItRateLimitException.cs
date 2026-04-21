using System.Net;

namespace TownCrier.Application.PlanIt;

/// <summary>
/// Thrown by the PlanIt client when the upstream API returns HTTP 429. Carries
/// the parsed <c>Retry-After</c> value so the polling cycle can schedule the
/// next run accordingly.
/// </summary>
public sealed class PlanItRateLimitException : HttpRequestException
{
    public PlanItRateLimitException(TimeSpan? retryAfter)
        : base(
            "PlanIt API returned 429 Too Many Requests.",
            inner: null,
            statusCode: HttpStatusCode.TooManyRequests)
    {
        this.RetryAfter = retryAfter;
    }

    public PlanItRateLimitException(TimeSpan? retryAfter, Exception? innerException)
        : base(
            "PlanIt API returned 429 Too Many Requests.",
            innerException,
            statusCode: HttpStatusCode.TooManyRequests)
    {
        this.RetryAfter = retryAfter;
    }

    public PlanItRateLimitException()
        : base(
            "PlanIt API returned 429 Too Many Requests.",
            inner: null,
            statusCode: HttpStatusCode.TooManyRequests)
    {
    }

    public PlanItRateLimitException(string? message)
        : base(message, inner: null, statusCode: HttpStatusCode.TooManyRequests)
    {
    }

    public PlanItRateLimitException(string? message, Exception? innerException)
        : base(message, innerException, statusCode: HttpStatusCode.TooManyRequests)
    {
    }

    public TimeSpan? RetryAfter { get; }
}
