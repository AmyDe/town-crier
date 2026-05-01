using System.Net;
using System.Net.Http.Headers;
using Microsoft.Extensions.DependencyInjection;
using TownCrier.Application.UserProfiles;
using TownCrier.Web.Tests.Auth;

namespace TownCrier.Web.Tests.UserProfiles;

public sealed class UpdateUserProfileEndpointTests
{
    [Test]
    public async Task Should_PersistSavedDecisionPushFalse_When_PatchSetsItFalse()
    {
        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", TestJwtToken.Generate());

        // Create the profile so the PATCH has something to update.
        (await client.PostAsync(new Uri("/v1/me", UriKind.Relative), null)).Dispose();

        // Act — request both saved-decision flags = false.
        using var content = new StringContent(
            """{"pushEnabled":true,"savedDecisionPush":false,"savedDecisionEmail":false}""",
            System.Text.Encoding.UTF8,
            "application/json");
        using var response = await client.PatchAsync(new Uri("/v1/me", UriKind.Relative), content);

        // Assert — endpoint must forward the new fields, not silently drop them.
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);

        using var scope = factory.Services.CreateScope();
        var repository = scope.ServiceProvider.GetRequiredService<IUserProfileRepository>();
        var saved = await repository.GetByUserIdAsync("auth0|test-user-123", CancellationToken.None);
        await Assert.That(saved).IsNotNull();
        await Assert.That(saved!.NotificationPreferences.SavedDecisionPush).IsFalse();
        await Assert.That(saved.NotificationPreferences.SavedDecisionEmail).IsFalse();
    }
}
