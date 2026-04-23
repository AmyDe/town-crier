using System.Net;

namespace TownCrier.Infrastructure.Tests.Cosmos;

internal sealed class ValidationHttpHandler : HttpMessageHandler
{
    private readonly Func<HttpRequestMessage, HttpResponseMessage> validator;

    public ValidationHttpHandler(Func<HttpRequestMessage, HttpResponseMessage> validator)
    {
        this.validator = validator;
    }

    protected override Task<HttpResponseMessage> SendAsync(
        HttpRequestMessage request,
        CancellationToken cancellationToken)
    {
        var response = this.validator(request);
        return Task.FromResult(response);
    }
}
