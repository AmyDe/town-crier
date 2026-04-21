using System.Diagnostics.CodeAnalysis;
using System.Net;
using System.Net.Http.Headers;
using System.Text;
using TownCrier.Application.PlanIt;
using TownCrier.Infrastructure.PlanIt;

namespace TownCrier.Infrastructure.Tests.PlanIt;

[SuppressMessage("Reliability", "CA2000:Dispose objects before losing scope", Justification = "HttpClient disposes the handler")]
[SuppressMessage("Minor Code Smell", "S1075:URIs should not be hardcoded", Justification = "Test base address")]
public sealed class PlanItRateLimitExceptionTests
{
    private const string BaseUrl = "https://www.planit.org.uk";

    [Test]
    public async Task Should_ThrowPlanItRateLimitException_With_ParsedRetryAfter_When_429HasDeltaSecondsHeader()
    {
        using var handler = new HeaderedFakeHandler(HttpStatusCode.TooManyRequests, retryAfterSeconds: 45);
        var client = CreateClient(handler);

        var ex = await Assert.ThrowsAsync<PlanItRateLimitException>(
            async () => await client.FetchApplicationsPageAsync(292, null, 1, CancellationToken.None));

        await Assert.That(ex.RetryAfter).IsEqualTo(TimeSpan.FromSeconds(45));
    }

    [Test]
    public async Task Should_ThrowPlanItRateLimitException_With_NullRetryAfter_When_429HasNoHeader()
    {
        using var handler = new HeaderedFakeHandler(HttpStatusCode.TooManyRequests, retryAfterSeconds: null);
        var client = CreateClient(handler);

        var ex = await Assert.ThrowsAsync<PlanItRateLimitException>(
            async () => await client.FetchApplicationsPageAsync(292, null, 1, CancellationToken.None));

        await Assert.That(ex.RetryAfter).IsNull();
    }

    private static PlanItClient CreateClient(HttpMessageHandler handler)
    {
        var httpClient = new HttpClient(handler, disposeHandler: false)
        {
            BaseAddress = new Uri(BaseUrl),
        };

        var throttleOptions = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0 };
        var retryOptions = new PlanItRetryOptions { MaxRetries = 0 };
        return new PlanItClient(httpClient, throttleOptions, retryOptions, (_, _) => Task.CompletedTask);
    }

    private sealed class HeaderedFakeHandler : HttpMessageHandler
    {
        private readonly HttpStatusCode statusCode;
        private readonly int? retryAfterSeconds;

        public HeaderedFakeHandler(HttpStatusCode statusCode, int? retryAfterSeconds)
        {
            this.statusCode = statusCode;
            this.retryAfterSeconds = retryAfterSeconds;
        }

        protected override Task<HttpResponseMessage> SendAsync(HttpRequestMessage request, CancellationToken cancellationToken)
        {
            var response = new HttpResponseMessage(this.statusCode)
            {
                Content = new StringContent("{}", Encoding.UTF8, "application/json"),
            };
            if (this.retryAfterSeconds is { } secs)
            {
                response.Headers.RetryAfter = new RetryConditionHeaderValue(TimeSpan.FromSeconds(secs));
            }

            return Task.FromResult(response);
        }
    }
}
