namespace TownCrier.Application.Polling;

public static class CycleTypeExtensions
{
    /// <summary>
    /// Maps a <see cref="CycleType"/> to the lowercase string value used as the
    /// <c>cycle.type</c> tag on polling metrics, spans, and structured logs.
    /// </summary>
    /// <param name="cycleType">The cycle type to convert.</param>
    /// <returns>The lowercase telemetry tag value for the given cycle type.</returns>
    /// <exception cref="ArgumentOutOfRangeException">
    /// Thrown when a new <see cref="CycleType"/> member is added without updating
    /// this mapping. Fail-fast prevents silent telemetry misattribution.
    /// </exception>
    public static string ToTelemetryValue(this CycleType cycleType) => cycleType switch
    {
        CycleType.Watched => "watched",
        CycleType.Seed => "seed",
        _ => throw new ArgumentOutOfRangeException(
            nameof(cycleType),
            cycleType,
            $"Unknown {nameof(CycleType)} value '{cycleType}'. Update {nameof(CycleTypeExtensions)}.{nameof(ToTelemetryValue)} when adding new cycle types."),
    };
}
