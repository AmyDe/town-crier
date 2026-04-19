using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class CycleTypeExtensionsTests
{
    [Test]
    public async Task Should_ReturnWatched_When_CycleTypeIsWatched()
    {
        // Arrange
        var cycleType = CycleType.Watched;

        // Act
        var result = cycleType.ToTelemetryValue();

        // Assert
        await Assert.That(result).IsEqualTo("watched");
    }

    [Test]
    public async Task Should_ReturnSeed_When_CycleTypeIsSeed()
    {
        // Arrange
        var cycleType = CycleType.Seed;

        // Act
        var result = cycleType.ToTelemetryValue();

        // Assert
        await Assert.That(result).IsEqualTo("seed");
    }

    [Test]
    public void Should_ThrowArgumentOutOfRangeException_When_CycleTypeIsUnknown()
    {
        // Arrange
        var unknownCycleType = (CycleType)999;

        // Act + Assert
        Assert.Throws<ArgumentOutOfRangeException>(() => unknownCycleType.ToTelemetryValue());
    }
}
