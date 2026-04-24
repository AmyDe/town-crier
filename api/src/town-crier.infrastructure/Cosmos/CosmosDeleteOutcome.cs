namespace TownCrier.Infrastructure.Cosmos;

public enum CosmosDeleteOutcome
{
    Deleted = 0,
    NotFound = 1,
    PreconditionFailed = 2,
}
