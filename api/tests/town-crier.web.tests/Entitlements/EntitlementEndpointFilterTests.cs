using System.Net;
using System.Net.Http.Headers;
using System.Security.Claims;
using TownCrier.Domain.Entitlements;
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

    [Test]
    public async Task Should_Return403_When_FreeTierAccessesSearch()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        var token = TestJwtToken.Generate(claims:
        [
            new Claim("subscription_tier", "Free"),
        ]);
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);

        var response = await client.GetAsync(new Uri("/v1/search?q=test&authorityId=42&page=1", UriKind.Relative));

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Forbidden);
    }

    [Test]
    public async Task Should_AllowSearch_When_ProTier()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        var token = TestJwtToken.Generate(
            userId: "auth0|pro-search-user",
            claims: [new Claim("subscription_tier", "Pro")]);
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);

        // Create user profile first so search handler doesn't 404
        await client.PostAsync(new Uri("/v1/me", UriKind.Relative), null);

        var response = await client.GetAsync(new Uri("/v1/search?q=test&authorityId=42&page=1", UriKind.Relative));

        // Should NOT be 403 — may be 200 with empty results
        await Assert.That(response.StatusCode).IsNotEqualTo(HttpStatusCode.Forbidden);
    }

    [Test]
    public async Task Should_DefaultToFree_When_TierClaimMissing()
    {
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        var token = TestJwtToken.Generate();
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", token);

        var response = await client.GetAsync(new Uri("/v1/search?q=test&authorityId=42&page=1", UriKind.Relative));

        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.Forbidden);
    }
}
