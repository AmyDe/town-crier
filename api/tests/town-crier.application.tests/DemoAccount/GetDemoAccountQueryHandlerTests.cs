using TownCrier.Application.DemoAccount;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.UserProfiles;
using TownCrier.Domain.UserProfiles;

namespace TownCrier.Application.Tests.DemoAccount;

public sealed class GetDemoAccountQueryHandlerTests
{
    [Test]
    public async Task Should_CreateDemoProfile_When_NoneExists()
    {
        // Arrange
        var userProfileRepository = new FakeUserProfileRepository();
        var watchZoneRepository = new FakeWatchZoneRepository();
        var planningApplicationRepository = new FakePlanningApplicationRepository();
        var handler = new GetDemoAccountQueryHandler(
            userProfileRepository, watchZoneRepository, planningApplicationRepository);

        // Act
        var result = await handler.HandleAsync(new GetDemoAccountQuery(), CancellationToken.None);

        // Assert
        await Assert.That(result.UserId).IsEqualTo("demo|apple-reviewer");
        await Assert.That(result.Tier).IsEqualTo(SubscriptionTier.Pro);
    }

    [Test]
    public async Task Should_ReturnExistingProfile_When_DemoAlreadyExists()
    {
        // Arrange
        var userProfileRepository = new FakeUserProfileRepository();
        var watchZoneRepository = new FakeWatchZoneRepository();
        var planningApplicationRepository = new FakePlanningApplicationRepository();
        var handler = new GetDemoAccountQueryHandler(
            userProfileRepository, watchZoneRepository, planningApplicationRepository);

        // Create first
        await handler.HandleAsync(new GetDemoAccountQuery(), CancellationToken.None);

        // Act — fetch again
        var result = await handler.HandleAsync(new GetDemoAccountQuery(), CancellationToken.None);

        // Assert — still one profile
        await Assert.That(userProfileRepository.Count).IsEqualTo(1);
        await Assert.That(result.UserId).IsEqualTo("demo|apple-reviewer");
    }

    [Test]
    public async Task Should_SeedWatchZone_When_CreatingDemoAccount()
    {
        // Arrange
        var userProfileRepository = new FakeUserProfileRepository();
        var watchZoneRepository = new FakeWatchZoneRepository();
        var planningApplicationRepository = new FakePlanningApplicationRepository();
        var handler = new GetDemoAccountQueryHandler(
            userProfileRepository, watchZoneRepository, planningApplicationRepository);

        // Act
        var result = await handler.HandleAsync(new GetDemoAccountQuery(), CancellationToken.None);

        // Assert
        await Assert.That(result.WatchZone).IsNotNull();
        await Assert.That(result.WatchZone.ZoneId).IsEqualTo("demo-zone");
        await Assert.That(result.WatchZone.AuthorityName).IsNotNull();
    }

    [Test]
    public async Task Should_SeedPlanningApplications_When_CreatingDemoAccount()
    {
        // Arrange
        var userProfileRepository = new FakeUserProfileRepository();
        var watchZoneRepository = new FakeWatchZoneRepository();
        var planningApplicationRepository = new FakePlanningApplicationRepository();
        var handler = new GetDemoAccountQueryHandler(
            userProfileRepository, watchZoneRepository, planningApplicationRepository);

        // Act
        var result = await handler.HandleAsync(new GetDemoAccountQuery(), CancellationToken.None);

        // Assert — should seed exactly the number of applications defined in DemoSeedData
        var expectedCount = DemoSeedData.CreateApplications(DateTimeOffset.UtcNow).Count;
        await Assert.That(result.Applications.Count).IsEqualTo(expectedCount);
    }

    [Test]
    public async Task Should_UseAuthorityFromSeedData_When_CreatingWatchZone()
    {
        // Arrange
        var userProfileRepository = new FakeUserProfileRepository();
        var watchZoneRepository = new FakeWatchZoneRepository();
        var planningApplicationRepository = new FakePlanningApplicationRepository();
        var handler = new GetDemoAccountQueryHandler(
            userProfileRepository, watchZoneRepository, planningApplicationRepository);

        // Act
        var result = await handler.HandleAsync(new GetDemoAccountQuery(), CancellationToken.None);

        // Assert — watch zone authority name should match the DemoSeedData constant
        await Assert.That(result.WatchZone.AuthorityName).IsEqualTo(DemoSeedData.AuthorityName);
    }

    [Test]
    public async Task Should_NotDuplicateSeededData_When_CalledMultipleTimes()
    {
        // Arrange
        var userProfileRepository = new FakeUserProfileRepository();
        var watchZoneRepository = new FakeWatchZoneRepository();
        var planningApplicationRepository = new FakePlanningApplicationRepository();
        var handler = new GetDemoAccountQueryHandler(
            userProfileRepository, watchZoneRepository, planningApplicationRepository);

        // Act
        await handler.HandleAsync(new GetDemoAccountQuery(), CancellationToken.None);
        var result = await handler.HandleAsync(new GetDemoAccountQuery(), CancellationToken.None);

        // Assert — same number of applications each time (idempotent)
        await Assert.That(result.Applications.Count).IsGreaterThanOrEqualTo(3);
        await Assert.That(planningApplicationRepository.GetAll().Count).IsGreaterThanOrEqualTo(3);
    }
}
