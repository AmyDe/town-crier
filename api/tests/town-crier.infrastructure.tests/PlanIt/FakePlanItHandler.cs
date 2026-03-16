using System.Net;
using System.Text;
using System.Text.Json;

namespace TownCrier.Infrastructure.Tests.PlanIt;

internal sealed class FakePlanItHandler : HttpMessageHandler
{
    private readonly Dictionary<string, string> responses = new();
    private readonly List<string> requestUrls = [];

    public IReadOnlyList<string> RequestUrls => this.requestUrls;

    public void SetupJsonResponse(string urlContains, string json)
    {
        this.responses[urlContains] = json;
    }

    protected override Task<HttpResponseMessage> SendAsync(
        HttpRequestMessage request,
        CancellationToken cancellationToken)
    {
        var url = request.RequestUri?.PathAndQuery ?? string.Empty;
        this.requestUrls.Add(url);

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
