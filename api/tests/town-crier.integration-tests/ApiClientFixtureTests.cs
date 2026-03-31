namespace TownCrier.IntegrationTests;

public sealed class ApiClientFixtureTests
{
    [Test]
    public async Task Should_ThrowInvalidOperationException_When_FixtureNotInitialized()
    {
        // Arrange
        await using var fixture = new ApiClientFixture();

        // Act & Assert
        Assert.Throws<InvalidOperationException>(
            () => _ = fixture.Client);
    }

    [Test]
    public async Task Should_ImplementIAsyncDisposable()
    {
        // Arrange
        var fixture = new ApiClientFixture();

        // Act & Assert -- should not throw
        await fixture.DisposeAsync();
    }
}
