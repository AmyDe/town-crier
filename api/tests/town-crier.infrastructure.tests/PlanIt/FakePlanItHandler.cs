using System.Net;
using System.Text;

namespace TownCrier.Infrastructure.Tests.PlanIt;

internal sealed class FakePlanItHandler : HttpMessageHandler
{
    private readonly Dictionary<string, string> responses = new();
    private readonly List<string> requestUrls = [];
    private readonly Dictionary<string, int> rateLimitCounters = new();
    private readonly Dictionary<string, HttpStatusCode> statusCodeResponses = new();
    private readonly Dictionary<string, (int RemainingFailures, HttpStatusCode StatusCode, string SuccessJson)> transientFailures = new();

    public IReadOnlyList<string> RequestUrls => this.requestUrls;

    public void SetupJsonResponse(string urlContains, string json)
    {
        this.responses[urlContains] = json;
    }

    public void SetupRateLimitForever(string urlContains)
    {
        this.rateLimitCounters[urlContains] = int.MaxValue;
    }

    public void SetupStatusCodeResponse(string urlContains, HttpStatusCode statusCode)
    {
        this.statusCodeResponses[urlContains] = statusCode;
    }

    public void SetupTransientFailure(string urlContains, int failCount, HttpStatusCode statusCode, string successJson)
    {
        this.transientFailures[urlContains] = (failCount, statusCode, successJson);
    }

    public void SetupTransientRateLimit(string urlContains, int failCount, string successJson)
    {
        this.transientFailures[urlContains] = (failCount, HttpStatusCode.TooManyRequests, successJson);
    }

    protected override Task<HttpResponseMessage> SendAsync(
        HttpRequestMessage request,
        CancellationToken cancellationToken)
    {
        var url = request.RequestUri?.PathAndQuery ?? string.Empty;
        this.requestUrls.Add(url);

        // Check transient failures first (they take priority)
        var transientKey = this.transientFailures.Keys.FirstOrDefault(
            k => url.Contains(k, StringComparison.Ordinal));
        if (transientKey is not null)
        {
            var (remaining, statusCode, successJson) = this.transientFailures[transientKey];
            if (remaining > 0)
            {
                this.transientFailures[transientKey] = (remaining - 1, statusCode, successJson);
                return Task.FromResult(new HttpResponseMessage(statusCode));
            }

            return Task.FromResult(new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new StringContent(successJson, Encoding.UTF8, "application/json"),
            });
        }

        foreach (var (key, _) in this.rateLimitCounters)
        {
            if (url.Contains(key, StringComparison.Ordinal) && this.rateLimitCounters[key] > 0)
            {
                this.rateLimitCounters[key]--;
                return Task.FromResult(new HttpResponseMessage((HttpStatusCode)429));
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
