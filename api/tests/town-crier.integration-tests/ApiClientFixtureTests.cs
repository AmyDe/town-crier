namespace TownCrier.IntegrationTests;

public sealed class ApiClientFixtureTests
{
    [Test]
    public async Task Should_ThrowInvalidOperationException_When_FixtureNotInitialized()
    {
        // Arrange
        var fixture = new ApiClientFixture();

        // Act
        var act = () => fixture.Client;

        // Assert
        await Assert.ThrowsAsync<InvalidOperationException>(
            () => Task.FromResult(act()));
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
