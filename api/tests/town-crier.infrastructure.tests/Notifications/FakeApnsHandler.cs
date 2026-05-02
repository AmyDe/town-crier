using System.Net;
using System.Text;

namespace TownCrier.Infrastructure.Tests.Notifications;

#pragma warning disable CA2000 // Responses are owned by callers via SendAsync.
internal sealed class FakeApnsHandler : HttpMessageHandler
{
    private readonly Queue<(HttpStatusCode Status, string? ReasonJson)> responses = new();
    private readonly Lock requestsGate = new();
    private readonly Lock responsesGate = new();
    private readonly List<RecordedRequest> sentRequests = [];
    private int concurrencyCounter;

    public IReadOnlyList<RecordedRequest> SentRequests
    {
        get
        {
            lock (this.requestsGate)
            {
                return [.. this.sentRequests];
            }
        }
    }

    public int Concurrency { get; private set; }

    public int PeakConcurrency { get; private set; }

    public TimeSpan ResponseDelay { get; set; } = TimeSpan.Zero;

    public void EnqueueOk()
    {
        lock (this.responsesGate)
        {
            this.responses.Enqueue((HttpStatusCode.OK, null));
        }
    }

    public void EnqueueRejection(HttpStatusCode status, string reason)
    {
        lock (this.responsesGate)
        {
            this.responses.Enqueue((status, $"{{\"reason\":\"{reason}\"}}"));
        }
    }

    public void EnqueueStatus(HttpStatusCode status)
    {
        lock (this.responsesGate)
        {
            this.responses.Enqueue((status, null));
        }
    }

    protected override async Task<HttpResponseMessage> SendAsync(
        HttpRequestMessage request,
        CancellationToken cancellationToken)
    {
        ArgumentNullException.ThrowIfNull(request);

        var current = Interlocked.Increment(ref this.concurrencyCounter);
        this.Concurrency = current;
        lock (this.requestsGate)
        {
            if (current > this.PeakConcurrency)
            {
                this.PeakConcurrency = current;
            }
        }

        try
        {
            var body = string.Empty;
            if (request.Content is not null)
            {
                body = await request.Content.ReadAsStringAsync(cancellationToken).ConfigureAwait(false);
            }

            var headers = new Dictionary<string, string>(StringComparer.OrdinalIgnoreCase);
            foreach (var header in request.Headers)
            {
                headers[header.Key] = string.Join(",", header.Value);
            }

            var auth = request.Headers.Authorization is null
                ? null
                : new RecordedAuth(request.Headers.Authorization.Scheme, request.Headers.Authorization.Parameter);
            var recorded = new RecordedRequest(
                request.Method,
                request.RequestUri,
                request.Version,
                request.VersionPolicy,
                auth,
                headers,
                body);
            lock (this.requestsGate)
            {
                this.sentRequests.Add(recorded);
            }

            if (this.ResponseDelay > TimeSpan.Zero)
            {
                await Task.Delay(this.ResponseDelay, cancellationToken).ConfigureAwait(false);
            }

            HttpStatusCode status;
            string? reasonJson;
            lock (this.responsesGate)
            {
                (status, reasonJson) = this.responses.Count > 0
                    ? this.responses.Dequeue()
                    : (HttpStatusCode.OK, null);
            }

            var response = new HttpResponseMessage(status);
            if (reasonJson is not null)
            {
                response.Content = new StringContent(reasonJson, Encoding.UTF8, "application/json");
            }

            return response;
        }
        finally
        {
            Interlocked.Decrement(ref this.concurrencyCounter);
        }
    }
}
#pragma warning restore CA2000
