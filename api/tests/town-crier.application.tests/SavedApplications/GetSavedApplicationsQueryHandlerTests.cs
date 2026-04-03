using TownCrier.Application.PlanningApplications;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.Tests.Polling;
using TownCrier.Domain.PlanningApplications;
using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.Tests.SavedApplications;

public sealed class GetSavedApplicationsQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnEmptyList_When_UserHasNoSavedApplications()
    {
        // Arrange
        var savedRepository = new FakeSavedApplicationRepository();
        var applicationRepository = new FakePlanningApplicationRepository();
        var handler = new GetSavedApplicationsQueryHandler(savedRepository, applicationRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(0);
    }

    [Test]
    public async Task Should_ReturnSavedApplicationsWithDetails_When_UserHasSaved()
    {
        // Arrange
        var savedRepository = new FakeSavedApplicationRepository();
        var applicationRepository = new FakePlanningApplicationRepository();
        var savedAt = new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero);

        var app1 = CreateApplication("planit-uid-abc", "APP/2026/001", "Wiltshire");
        var app2 = CreateApplication("planit-uid-def", "APP/2026/002", "Somerset");
        await applicationRepository.UpsertAsync(app1, CancellationToken.None);
        await applicationRepository.UpsertAsync(app2, CancellationToken.None);

        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|user-1", "planit-uid-abc", savedAt), CancellationToken.None);
        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|user-1", "planit-uid-def", savedAt.AddHours(1)), CancellationToken.None);

        var handler = new GetSavedApplicationsQueryHandler(savedRepository, applicationRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(2);
        await Assert.That(result[0].ApplicationUid).IsEqualTo("planit-uid-abc");
        await Assert.That(result[0].Application.Name).IsEqualTo("APP/2026/001");
        await Assert.That(result[0].Application.AreaName).IsEqualTo("Wiltshire");
        await Assert.That(result[1].ApplicationUid).IsEqualTo("planit-uid-def");
        await Assert.That(result[1].Application.Name).IsEqualTo("APP/2026/002");
    }

    [Test]
    public async Task Should_OnlyReturnOwnApplications_When_MultipleUsersHaveSaved()
    {
        // Arrange
        var savedRepository = new FakeSavedApplicationRepository();
        var applicationRepository = new FakePlanningApplicationRepository();
        var savedAt = DateTimeOffset.UtcNow;

        var app1 = CreateApplication("planit-uid-abc", "APP/2026/001", "Wiltshire");
        var app2 = CreateApplication("planit-uid-def", "APP/2026/002", "Somerset");
        await applicationRepository.UpsertAsync(app1, CancellationToken.None);
        await applicationRepository.UpsertAsync(app2, CancellationToken.None);

        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|user-1", "planit-uid-abc", savedAt), CancellationToken.None);
        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|user-2", "planit-uid-def", savedAt), CancellationToken.None);

        var handler = new GetSavedApplicationsQueryHandler(savedRepository, applicationRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(1);
        await Assert.That(result[0].ApplicationUid).IsEqualTo("planit-uid-abc");
    }

    [Test]
    public async Task Should_ExcludeSavedApplication_When_PlanningApplicationNoLongerExists()
    {
        // Arrange
        var savedRepository = new FakeSavedApplicationRepository();
        var applicationRepository = new FakePlanningApplicationRepository();
        var savedAt = new DateTimeOffset(2026, 3, 17, 10, 0, 0, TimeSpan.Zero);

        var app = CreateApplication("planit-uid-abc", "APP/2026/001", "Wiltshire");
        await applicationRepository.UpsertAsync(app, CancellationToken.None);

        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|user-1", "planit-uid-abc", savedAt), CancellationToken.None);
        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|user-1", "planit-uid-orphaned", savedAt.AddHours(1)), CancellationToken.None);

        var handler = new GetSavedApplicationsQueryHandler(savedRepository, applicationRepository);
        var query = new GetSavedApplicationsQuery("auth0|user-1");

        // Act
        var result = await handler.HandleAsync(query, CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(1);
        await Assert.That(result[0].ApplicationUid).IsEqualTo("planit-uid-abc");
    }

    private static PlanningApplication CreateApplication(string uid, string name, string areaName)
    {
        return new PlanningApplication(
            name: name,
            uid: uid,
            areaName: areaName,
            areaId: 1,
            address: "1 Test Street",
            postcode: "BA1 1AA",
            description: "Test application",
            appType: "Full",
            appState: "Undecided",
            appSize: null,
            startDate: new DateOnly(2026, 1, 15),
            decidedDate: null,
            consultedDate: null,
            longitude: -2.36,
            latitude: 51.38,
            url: null,
            link: null,
            lastDifferent: DateTimeOffset.UtcNow);
    }
}
