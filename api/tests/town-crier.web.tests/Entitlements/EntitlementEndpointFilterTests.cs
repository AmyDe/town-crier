using System.Net;
using System.Net.Http.Headers;
using System.Security.Claims;
using TownCrier.Web.Tests.Auth;

namespace TownCrier.Web.Tests.Entitlements;

public sealed class EntitlementEndpointFilterTests
{
    [Test]
    public async Task Should_AllowPreferenceUpdate_When_FreeTier()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        var token = TestJwtToken.Generate(
            userId: "auth0|free-user-2",
            claims: [new Claim("subscription_tier", "Free")]);
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);

        (await client.PostAsync(new Uri("/v1/me", UriKind.Relative), null)).Dispose();

        // Free user can update preferences
        using var content = new StringContent(
            """{"pushEnabled":false,"emailDigestEnabled":false}""",
            System.Text.Encoding.UTF8,
            "application/json");
        using var response = await client.PatchAsync(new Uri("/v1/me", UriKind.Relative), content);

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);
    }
}
