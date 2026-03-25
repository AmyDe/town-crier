namespace TownCrier.Application.VersionConfig;

public static class GetVersionConfigQueryHandler
{
    public static Task<GetVersionConfigResult> HandleAsync(GetVersionConfigQuery query, CancellationToken ct)
    {
        return Task.FromResult(new GetVersionConfigResult(MinimumVersion: "1.0.0"));
    }
}
