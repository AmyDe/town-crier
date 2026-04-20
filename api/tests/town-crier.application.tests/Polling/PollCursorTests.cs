using TownCrier.Application.Polling;

namespace TownCrier.Application.Tests.Polling;

public sealed class PollCursorTests
{
    [Test]
    public async Task Should_ExposeConstructorArguments_AsProperties()
    {
        // Arrange
        var differentStart = new DateOnly(2026, 4, 18);
        const int nextPage = 4;
        const int knownTotal = 7200;

        // Act
        var cursor = new PollCursor(differentStart, nextPage, knownTotal);

        // Assert
        await Assert.That(cursor.DifferentStart).IsEqualTo(differentStart);
        await Assert.That(cursor.NextPage).IsEqualTo(nextPage);
        await Assert.That(cursor.KnownTotal).IsEqualTo(knownTotal);
    }

    [Test]
    public async Task Should_AllowNullKnownTotal()
    {
        // Arrange
        var differentStart = new DateOnly(2026, 4, 18);

        // Act
        var cursor = new PollCursor(differentStart, NextPage: 2, KnownTotal: null);

        // Assert
        await Assert.That(cursor.KnownTotal).IsNull();
    }

    [Test]
    public async Task Should_BeValueEqual_When_FieldsMatch()
    {
        // Arrange
        var a = new PollCursor(new DateOnly(2026, 4, 18), NextPage: 4, KnownTotal: 7200);
        var b = new PollCursor(new DateOnly(2026, 4, 18), NextPage: 4, KnownTotal: 7200);

        // Act + Assert
        await Assert.That(a).IsEqualTo(b);
    }
}
