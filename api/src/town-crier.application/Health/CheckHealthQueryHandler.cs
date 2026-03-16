namespace TownCrier.Application.Health;

public static class CheckHealthQueryHandler
{
    public static Task<HealthStatus> HandleAsync(CheckHealthQuery query, CancellationToken ct)
    {
        return Task.FromResult(new HealthStatus("Healthy"));
    }
}
