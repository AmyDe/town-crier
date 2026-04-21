namespace TownCrier.Infrastructure.Polling;

/// <summary>
/// Payload for the Service Bus poll trigger message. The body has no semantic
/// content — the message is a tick that tells the worker "run once now" — but
/// we keep a timestamp for diagnostics so an operator reading the DLQ can see
/// when the chain last published.
/// </summary>
internal sealed record PollTriggerPayload
{
    public required string PublishedAtUtc { get; init; }
}
