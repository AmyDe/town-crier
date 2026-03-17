using System.Net;
using System.Text;

namespace TownCrier.Infrastructure.Tests.GovUkPlanningData;

internal sealed class FakeGovUkHandler : HttpMessageHandler
{
    private readonly Dictionary<string, (HttpStatusCode StatusCode, string Body)> responses = new();
    private readonly List<string> requestUrls = [];

    public IReadOnlyList<string> RequestUrls => this.requestUrls;

    public void SetupJsonResponse(string urlContains, string json)
    {
        this.responses[urlContains] = (HttpStatusCode.OK, json);
    }

    public void SetupErrorResponse(string urlContains, HttpStatusCode statusCode)
    {
        this.responses[urlContains] = (statusCode, string.Empty);
    }

    protected override Task<HttpResponseMessage> SendAsync(
        HttpRequestMessage request,
        CancellationToken cancellationToken)
    {
        var url = request.RequestUri?.PathAndQuery ?? string.Empty;
        this.requestUrls.Add(url);

        foreach (var (key, (statusCode, body)) in this.responses)
        {
            if (url.Contains(key, StringComparison.Ordinal))
            {
                var response = new HttpResponseMessage(statusCode);
                if (!string.IsNullOrEmpty(body))
                {
                    response.Content = new StringContent(body, Encoding.UTF8, "application/json");
                }

                return Task.FromResult(response);
            }
        }

        return Task.FromResult(new HttpResponseMessage(HttpStatusCode.NotFound));
    }
}
