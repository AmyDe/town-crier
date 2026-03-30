using System.Net;
using System.Text.Json;

namespace TownCrier.IntegrationTests;

[NotInParallel]
public sealed class Auth0TokenProviderTests
{
    [Test]
    public async Task Should_IncludeClientSecret_When_ClientSecretProvided()
    {
        // Arrange
        SetAllRequiredEnvVars();
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_CLIENT_SECRET", "test-secret");

        Dictionary<string, string>? capturedPayload = null;
        var handler = new FakeHttpMessageHandler(async request =>
        {
            var content = await request.Content!.ReadAsStringAsync();
            capturedPayload = JsonSerializer.Deserialize<Dictionary<string, string>>(content);
            return new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new StringContent("""{"access_token":"fake-token"}""",
                    System.Text.Encoding.UTF8, "application/json"),
            };
        });

        using var client = new HttpClient(handler);

        // Act
        var token = await Auth0TokenProvider.AcquireTokenAsync(client);

        // Assert
        await Assert.That(token).IsEqualTo("fake-token");
        await Assert.That(capturedPayload).IsNotNull();
        await Assert.That(capturedPayload!.ContainsKey("client_secret")).IsTrue();
        await Assert.That(capturedPayload["client_secret"]).IsEqualTo("test-secret");
        await Assert.That(capturedPayload["grant_type"]).IsEqualTo("password");

        // Cleanup
        CleanupEnvVars();
    }

    [Test]
    public async Task Should_ExcludeClientSecret_When_ClientSecretNotProvided()
    {
        // Arrange
        SetAllRequiredEnvVars();
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_CLIENT_SECRET", null);

        Dictionary<string, string>? capturedPayload = null;
        var handler = new FakeHttpMessageHandler(async request =>
        {
            var content = await request.Content!.ReadAsStringAsync();
            capturedPayload = JsonSerializer.Deserialize<Dictionary<string, string>>(content);
            return new HttpResponseMessage(HttpStatusCode.OK)
            {
                Content = new StringContent("""{"access_token":"fake-token"}""",
                    System.Text.Encoding.UTF8, "application/json"),
            };
        });

        using var client = new HttpClient(handler);

        // Act
        var token = await Auth0TokenProvider.AcquireTokenAsync(client);

        // Assert
        await Assert.That(token).IsEqualTo("fake-token");
        await Assert.That(capturedPayload).IsNotNull();
        await Assert.That(capturedPayload!.ContainsKey("client_secret")).IsFalse();

        // Cleanup
        CleanupEnvVars();
    }

    [Test]
    public async Task Should_ThrowInvalidOperationException_When_Auth0ReturnsError()
    {
        // Arrange
        SetAllRequiredEnvVars();
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_CLIENT_SECRET", null);

        var handler = new FakeHttpMessageHandler(_ =>
            Task.FromResult(new HttpResponseMessage(HttpStatusCode.Unauthorized)
            {
                Content = new StringContent("""{"error":"invalid_grant"}""",
                    System.Text.Encoding.UTF8, "application/json"),
            }));

        using var client = new HttpClient(handler);

        // Act & Assert
        await Assert.ThrowsAsync<InvalidOperationException>(
            () => Auth0TokenProvider.AcquireTokenAsync(client));

        // Cleanup
        CleanupEnvVars();
    }

    private static void SetAllRequiredEnvVars()
    {
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_API_BASE_URL", "https://test.example.com");
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_DOMAIN", "test.auth0.com");
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_CLIENT_ID", "test-client-id");
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_AUDIENCE", "https://api.test.com");
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_USERNAME", "user@test.com");
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_PASSWORD", "test-password");
    }

    private static void CleanupEnvVars()
    {
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_API_BASE_URL", null);
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_DOMAIN", null);
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_CLIENT_ID", null);
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_CLIENT_SECRET", null);
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_AUTH0_AUDIENCE", null);
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_USERNAME", null);
        Environment.SetEnvironmentVariable("INTEGRATION_TEST_PASSWORD", null);
    }
}
