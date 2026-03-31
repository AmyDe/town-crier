using System.Net;
using System.Text.Json;

namespace TownCrier.IntegrationTests;

[NotInParallel]
public sealed class UserProfileTests(ApiClientFixture fixture)
{
    [Test]
    [ClassDataSource<ApiClientFixture>(Shared = SharedType.Globally)]
    public async Task Should_CreateAndRetrieveProfile_When_Authenticated()
    {
        // Arrange
        var client = fixture.Client;

        // Act -- create profile
        using var createResponse = await client
            .PostAsync(new Uri("/v1/me", UriKind.Relative), null)
            .ConfigureAwait(false);

        // Assert -- create returns 200
        await Assert.That(createResponse.StatusCode).IsEqualTo(HttpStatusCode.OK);

        // Act -- retrieve profile
        using var getResponse = await client
            .GetAsync(new Uri("/v1/me", UriKind.Relative))
            .ConfigureAwait(false);
        var body = await getResponse.Content.ReadAsStringAsync().ConfigureAwait(false);

        // Assert -- retrieve returns 200 with userId
        await Assert.That(getResponse.StatusCode).IsEqualTo(HttpStatusCode.OK);

        using var doc = JsonDocument.Parse(body);
        var userId = doc.RootElement.GetProperty("userId").GetString();
        await Assert.That(userId).IsNotNullOrEmpty();
    }
}
