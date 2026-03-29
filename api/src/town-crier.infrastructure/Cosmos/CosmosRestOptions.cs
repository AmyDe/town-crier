namespace TownCrier.Infrastructure.Cosmos;

public sealed class CosmosRestOptions
{
    public required string AccountEndpoint { get; init; }

    public required string DatabaseName { get; init; }
}
