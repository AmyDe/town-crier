using System.Net.Http.Headers;
using Microsoft.Extensions.DependencyInjection;
using TownCrier.Application.UserProfiles;
using TownCrier.Domain.UserProfiles;
using TownCrier.Web.Tests.Auth;

namespace TownCrier.Web.Tests.UserProfiles;

public sealed class RecordUserActivityMiddlewareTests
{
    [Test]
    public async Task Should_UpdateLastActiveAt_When_AuthenticatedRequestCompletes()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var scope = factory.Services.CreateScope();
        var repository = scope.ServiceProvider.GetRequiredService<IUserProfileRepository>();

        // Seed a profile with LastActiveAt well in the past so the middleware writes.
        var stale = new DateTimeOffset(2025, 1, 1, 0, 0, 0, TimeSpan.Zero);
        var profile = UserProfile.Register("auth0|test-user-123", email: null, now: stale);
        await repository.SaveAsync(profile, CancellationToken.None);

        using var client = factory.CreateClient();
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", TestJwtToken.Generate());

        // Act — any authenticated endpoint triggers the middleware.
        using var response = await client.GetAsync(new Uri("/v1/me", UriKind.Relative));

        // Assert
        var after = await repository.GetByUserIdAsync("auth0|test-user-123", CancellationToken.None);
        await Assert.That(after).IsNotNull();
        await Assert.That(after!.LastActiveAt > stale).IsTrue();
    }

    [Test]
    public async Task Should_NotFail_When_UnauthenticatedRequest()
    {
        // Arrange — health endpoint is anonymous; middleware must not throw.
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();

        // Act
        using var response = await client.GetAsync(new Uri("/health", UriKind.Relative));

        // Assert
        await Assert.That(response.IsSuccessStatusCode).IsTrue();
    }
}
