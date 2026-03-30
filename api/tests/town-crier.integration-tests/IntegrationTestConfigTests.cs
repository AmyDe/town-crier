namespace TownCrier.IntegrationTests;

public sealed class IntegrationTestConfigTests
{
    [Test]
    public async Task Should_ThrowInvalidOperationException_When_RequiredEnvVarMissing()
    {
        // Arrange -- ensure the env var is not set
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_API_BASE_URL", null);

        // Act & Assert
        await Assert.ThrowsAsync<InvalidOperationException>(
            () => Task.FromResult(IntegrationTestConfig.ApiBaseUrl));
    }
}
