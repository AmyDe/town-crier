namespace TownCrier.Application.DemoAccount;

public sealed record DemoApplicationResult(
    string Uid,
    string Name,
    string Address,
    string Description,
    string? AppType,
    string? AppState);
