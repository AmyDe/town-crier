namespace TownCrier.Web.Endpoints;

internal sealed class AdminApiKeyFilter : IEndpointFilter
{
    private const string ApiKeyHeaderName = "X-Admin-Key";

    private readonly string expectedApiKey;

    public AdminApiKeyFilter(IConfiguration configuration)
    {
        this.expectedApiKey = configuration["Admin:ApiKey"]
            ?? throw new InvalidOperationException("Admin:ApiKey configuration is required.");
    }

    public async ValueTask<object?> InvokeAsync(
        EndpointFilterInvocationContext context,
        EndpointFilterDelegate next)
    {
        if (!context.HttpContext.Request.Headers.TryGetValue(ApiKeyHeaderName, out var providedKey)
            || !string.Equals(this.expectedApiKey, providedKey.ToString(), StringComparison.Ordinal))
        {
            return Results.Unauthorized();
        }

        return await next(context).ConfigureAwait(false);
    }
}
