using TownCrier.Application.DemoAccount;

namespace TownCrier.Application.Tests.DemoAccount;

public sealed class DemoSeedDataTests
{
    [Test]
    public async Task Should_HaveValidCoordinates_When_SeedDataCreated()
    {
        // Arrange & Act
        var applications = DemoSeedData.CreateApplications(DateTimeOffset.UtcNow);

        // Assert — every demo application must have non-null latitude and longitude
        foreach (var app in applications)
        {
            await Assert.That(app.Latitude).IsNotNull();
            await Assert.That(app.Longitude).IsNotNull();
        }
    }
}
