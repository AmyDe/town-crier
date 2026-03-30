namespace TownCrier.IntegrationTests;

[NotInParallel]
public sealed class IntegrationTestConfigTests
{
    [Test]
    public void Should_ThrowInvalidOperationException_When_RequiredEnvVarMissing()
    {
        // Arrange -- ensure the env var is not set
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_API_BASE_URL", null);

        // Act & Assert
        Assert.Throws<InvalidOperationException>(
            () => _ = IntegrationTestConfig.ApiBaseUrl);
    }

    [Test]
    public async Task Should_ReturnEnvVarValue_When_RequiredEnvVarIsSet()
    {
        // Arrange
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_API_BASE_URL", "https://test.example.com");
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_DOMAIN", "test.auth0.com");
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_CLIENT_ID", "test-client-id");
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_AUDIENCE", "https://api.test.com");
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_USERNAME", "user@test.com");
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_PASSWORD", "test-password");

        try
        {
            // Act & Assert
            await Assert.That(IntegrationTestConfig.ApiBaseUrl).IsEqualTo("https://test.example.com");
            await Assert.That(IntegrationTestConfig.Auth0Domain).IsEqualTo("test.auth0.com");
            await Assert.That(IntegrationTestConfig.Auth0ClientId).IsEqualTo("test-client-id");
            await Assert.That(IntegrationTestConfig.Auth0Audience).IsEqualTo("https://api.test.com");
            await Assert.That(IntegrationTestConfig.Username).IsEqualTo("user@test.com");
            await Assert.That(IntegrationTestConfig.Password).IsEqualTo("test-password");
        }
        finally
        {
            // Cleanup
            Environment.SetEnvironmentVariable("INTEGRATION_TEST_API_BASE_URL", null);
            Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_DOMAIN", null);
            Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_CLIENT_ID", null);
            Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_AUDIENCE", null);
            Environment.SetEnvironmentVariable("INTEGRATION_TEST_USERNAME", null);
            Environment.SetEnvironmentVariable("INTEGRATION_TEST_PASSWORD", null);
        }
    }

    [Test]
    public async Task Should_ReturnNull_When_OptionalClientSecretNotSet()
    {
        // Arrange -- ensure the optional env var is not set
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_CLIENT_SECRET", null);

        // Act
        var result = IntegrationTestConfig.Auth0ClientSecret;

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_ReturnValue_When_OptionalClientSecretIsSet()
    {
        // Arrange
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_CLIENT_SECRET", "my-secret");

        try
        {
            // Act
            var result = IntegrationTestConfig.Auth0ClientSecret;

            // Assert
            await Assert.That(result).IsEqualTo("my-secret");
        }
        finally
        {
            Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_CLIENT_SECRET", null);
        }
    }
}
