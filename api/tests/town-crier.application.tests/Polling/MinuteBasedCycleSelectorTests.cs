using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class MinuteBasedCycleSelectorTests
{
    [Test]
    [Arguments(0, CycleType.Watched)]
    [Arguments(14, CycleType.Watched)]
    [Arguments(15, CycleType.Seed)]
    [Arguments(29, CycleType.Seed)]
    [Arguments(30, CycleType.Watched)]
    [Arguments(44, CycleType.Watched)]
    [Arguments(45, CycleType.Seed)]
    [Arguments(59, CycleType.Seed)]
    public async Task Should_ReturnExpectedCycleType_When_MinuteIsAtBoundary(int minute, CycleType expected)
    {
        // Arrange
        var timeProvider = new FakeTimeProvider(new DateTimeOffset(2026, 4, 18, 12, minute, 0, TimeSpan.Zero));
        var selector = new MinuteBasedCycleSelector(timeProvider);

        // Act
        var result = selector.GetCurrent();

        // Assert
        await Assert.That(result).IsEqualTo(expected);
    }
}
