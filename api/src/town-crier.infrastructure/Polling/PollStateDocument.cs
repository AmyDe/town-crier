namespace TownCrier.Infrastructure.Polling;

internal sealed class PollStateDocument
{
    public required string Id { get; init; }

    public required string LastPollTime { get; init; }

    public int? AuthorityId { get; init; }
}
