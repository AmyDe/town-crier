using System.Net;
using System.Text;
using System.Text.Json;

namespace TownCrier.Infrastructure.Tests.PlanIt;

internal sealed class FakePlanItHandler : HttpMessageHandler
{
    private readonly Dictionary<string, string> responses = new();
    private readonly List<string> requestUrls = [];
    private readonly Dictionary<string, int> rateLimitCounters = new();
    private readonly Dictionary<string, string?> retryAfterHeaders = new();
    private readonly Dictionary<string, HttpStatusCode> statusCodeResponses = new();

    public IReadOnlyList<string> RequestUrls => this.requestUrls;

    public void SetupJsonResponse(string urlContains, string json)
    {
        this.responses[urlContains] = json;
    }

    public void SetupRateLimitThenSuccess(string urlContains, int count, string json)
    {
        this.rateLimitCounters[urlContains] = count;
        this.responses[urlContains] = json;
    }

    public void SetupRateLimitWithRetryAfter(string urlContains, int count, string successJson, string retryAfterValue)
    {
        this.rateLimitCounters[urlContains] = count;
        this.retryAfterHeaders[urlContains] = retryAfterValue;
        this.responses[urlContains] = successJson;
    }

    public void SetupRateLimitForever(string urlContains)
    {
        this.rateLimitCounters[urlContains] = int.MaxValue;
    }

    public void SetupStatusCodeResponse(string urlContains, HttpStatusCode statusCode)
    {
        this.statusCodeResponses[urlContains] = statusCode;
    }

    protected override Task<HttpResponseMessage> SendAsync(
        HttpRequestMessage request,
        CancellationToken cancellationToken)
    {
        var url = request.RequestUri?.PathAndQuery ?? string.Empty;
        this.requestUrls.Add(url);

        foreach (var (key, _) in this.rateLimitCounters)
        {
            if (url.Contains(key, StringComparison.Ordinal) && this.rateLimitCounters[key] > 0)
            {
                this.rateLimitCounters[key]--;
                var rateLimitResponse = new HttpResponseMessage((HttpStatusCode)429);
                if (this.retryAfterHeaders.TryGetValue(key, out var retryAfterValue) && retryAfterValue is not null)
                {
                    rateLimitResponse.Headers.TryAddWithoutValidation("Retry-After", retryAfterValue);
                }

                return Task.FromResult(rateLimitResponse);
            }
        }

        foreach (var (key, statusCode) in this.statusCodeResponses)
        {
            if (url.Contains(key, StringComparison.Ordinal))
            {
                return Task.FromResult(new HttpResponseMessage(statusCode));
            }
        }

        foreach (var (key, json) in this.responses)
        {
            if (url.Contains(key, StringComparison.Ordinal))
            {
                return Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK)
                {
                    Content = new StringContent(json, Encoding.UTF8, "application/json"),
                });
            }
        }

        return Task.FromResult(new HttpResponseMessage(HttpStatusCode.NotFound));
    }
}
