namespace Tc.Json;

internal sealed class GenerateOfferCodesRequest
{
    public required int Count { get; init; }

    public required string Tier { get; init; }

    public required int DurationDays { get; init; }
}
