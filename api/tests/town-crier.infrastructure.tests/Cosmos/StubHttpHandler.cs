using System.Net;
using System.Text;

namespace TownCrier.Infrastructure.Tests.Cosmos;

#pragma warning disable CA2000 // Responses are owned by callers via SendAsync
internal sealed class StubHttpHandler : HttpMessageHandler
{
    private readonly Queue<HttpResponseMessage> responses = new();

    public List<HttpRequestMessage> SentRequests { get; } = [];

    public void EnqueueResponse(
        HttpStatusCode statusCode,
        string? content = null,
        IEnumerable<KeyValuePair<string, string>>? headers = null)
    {
        var response = new HttpResponseMessage(statusCode);
        if (content is not null)
        {
            response.Content = new StringContent(content, Encoding.UTF8, "application/json");
        }

        if (headers is not null)
        {
            foreach (var (key, value) in headers)
            {
                response.Headers.TryAddWithoutValidation(key, value);
            }
        }

        this.responses.Enqueue(response);
    }

    protected override Task<HttpResponseMessage> SendAsync(
        HttpRequestMessage request,
        CancellationToken cancellationToken)
    {
        this.SentRequests.Add(request);
        return Task.FromResult(this.responses.Count > 0
            ? this.responses.Dequeue()
            : new HttpResponseMessage(HttpStatusCode.InternalServerError));
    }
}
#pragma warning restore CA2000
