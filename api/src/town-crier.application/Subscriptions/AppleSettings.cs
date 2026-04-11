namespace TownCrier.Application.Subscriptions;

/// <summary>
/// Configuration for Apple App Store integration.
/// </summary>
public sealed class AppleSettings
{
    public required string BundleId { get; init; }

    public required string Environment { get; init; }
}
