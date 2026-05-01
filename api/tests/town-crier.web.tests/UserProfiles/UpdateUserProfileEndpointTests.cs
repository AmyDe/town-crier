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

    [Test]
    public async Task Should_AcceptDigestDayAsString_When_PatchBodyUsesEnumName()
    {
        // Regression for tc-5gek: iOS sends digestDay as the enum name ("Wednesday")
        // because Swift's JSONEncoder serialises String-backed enums by raw value. The
        // API must accept that shape (string) and round-trip it back as a string on
        // GET — otherwise the iOS app's optimistic toggle update fails and snaps back.

        // Arrange
        await using var factory = new TestWebApplicationFactory();
        using var client = factory.CreateClient();
        client.DefaultRequestHeaders.Authorization = new AuthenticationHeaderValue("Bearer", TestJwtToken.Generate());

        (await client.PostAsync(new Uri("/v1/me", UriKind.Relative), null)).Dispose();

        // Act — body includes digestDay as a JSON string, mirroring the iOS client.
        using var content = new StringContent(
            """{"pushEnabled":true,"digestDay":"Wednesday","emailDigestEnabled":true,"savedDecisionPush":false,"savedDecisionEmail":false}""",
            System.Text.Encoding.UTF8,
            "application/json");
        using var response = await client.PatchAsync(new Uri("/v1/me", UriKind.Relative), content);

        // Assert — request must succeed and persist the named day.
        await Assert.That(response.StatusCode).IsEqualTo(HttpStatusCode.OK);

        using var scope = factory.Services.CreateScope();
        var repository = scope.ServiceProvider.GetRequiredService<IUserProfileRepository>();
        var saved = await repository.GetByUserIdAsync("auth0|test-user-123", CancellationToken.None);
        await Assert.That(saved).IsNotNull();
        await Assert.That(saved!.NotificationPreferences.DigestDay).IsEqualTo(DayOfWeek.Wednesday);

        // Assert — GET must echo the string form ("Wednesday") so iOS can decode it.
        using var getResponse = await client.GetAsync(new Uri("/v1/me", UriKind.Relative));
        await Assert.That(getResponse.StatusCode).IsEqualTo(HttpStatusCode.OK);
        var getBody = await getResponse.Content.ReadAsStringAsync();
        await Assert.That(getBody).Contains("\"digestDay\":\"Wednesday\"");
    }
}
