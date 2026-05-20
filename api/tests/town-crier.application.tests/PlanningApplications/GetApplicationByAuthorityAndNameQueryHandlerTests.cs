using TownCrier.Application.PlanningApplications;
using TownCrier.Application.Tests.Polling;
using TownCrier.Application.Tests.SavedApplications;
using TownCrier.Domain.SavedApplications;

namespace TownCrier.Application.Tests.PlanningApplications;

public sealed class GetApplicationByAuthorityAndNameQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnApplication_When_FoundByAuthorityAndName()
    {
        // Arrange — the handler does a partitioned point read (id = name, pk = authorityCode).
        // Authority code is AreaId.ToString() as stored in Cosmos.
        var application = new PlanningApplicationBuilder()
            .WithName("APP/2024/001")
            .WithAreaId(42)
            .WithAreaName("Camden")
            .WithUid("cam-001")
            .Build();

        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(application, CancellationToken.None);

        var savedRepository = new FakeSavedApplicationRepository();
        var handler = new GetApplicationByAuthorityAndNameQueryHandler(repository, savedRepository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByAuthorityAndNameQuery(
                AuthorityCode: "42",
                Name: "APP/2024/001",
                UserId: null),
            CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Name).IsEqualTo("APP/2024/001");
        await Assert.That(result.AreaId).IsEqualTo(42);
        await Assert.That(result.Uid).IsEqualTo("cam-001");
    }

    [Test]
    public async Task Should_ReturnNull_When_ApplicationNotFound()
    {
        // Arrange
        var repository = new FakePlanningApplicationRepository();
        var savedRepository = new FakeSavedApplicationRepository();
        var handler = new GetApplicationByAuthorityAndNameQueryHandler(repository, savedRepository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByAuthorityAndNameQuery(
                AuthorityCode: "42",
                Name: "NONEXISTENT/001",
                UserId: null),
            CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_RefreshSavedSnapshot_When_UserHasApplicationSaved()
    {
        // Arrange — same refresh-on-tap behaviour as the uid endpoint.
        var savedAt = new DateTimeOffset(2026, 5, 1, 10, 0, 0, TimeSpan.Zero);
        var stale = new PlanningApplicationBuilder()
            .WithName("APP/2024/001").WithAreaId(42).WithUid("cam-001").WithAppState("Undecided").Build();
        var fresh = new PlanningApplicationBuilder()
            .WithName("APP/2024/001").WithAreaId(42).WithUid("cam-001").WithAppState("Permitted").Build();

        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(fresh, CancellationToken.None);

        var savedRepository = new FakeSavedApplicationRepository();
        await savedRepository.SaveAsync(
            SavedApplication.Create("auth0|user-1", stale, savedAt), CancellationToken.None);

        var handler = new GetApplicationByAuthorityAndNameQueryHandler(repository, savedRepository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByAuthorityAndNameQuery(
                AuthorityCode: "42",
                Name: "APP/2024/001",
                UserId: "auth0|user-1"),
            CancellationToken.None);

        // Assert — caller sees fresh snapshot
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.AppState).IsEqualTo("Permitted");

        // Saved row updated with fresh snapshot
        var rows = await savedRepository.GetByUserIdAsync("auth0|user-1", CancellationToken.None);
        await Assert.That(rows).HasCount().EqualTo(1);
        await Assert.That(rows[0].Application).IsNotNull();
        await Assert.That(rows[0].Application!.AppState).IsEqualTo("Permitted");
        await Assert.That(rows[0].SavedAt).IsEqualTo(savedAt);
    }

    [Test]
    public async Task Should_UsePartitionedLookup_NotCrossPartitionScan()
    {
        // Arrange — verify the handler calls GetByAuthorityAndNameAsync (partitioned),
        // not GetByUidAsync (cross-partition). The FakePlanningApplicationRepository
        // tracks which overload is called.
        var application = new PlanningApplicationBuilder()
            .WithName("APP/2024/001").WithAreaId(42).Build();

        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(application, CancellationToken.None);

        var savedRepository = new FakeSavedApplicationRepository();
        var handler = new GetApplicationByAuthorityAndNameQueryHandler(repository, savedRepository);

        // Act
        await handler.HandleAsync(
            new GetApplicationByAuthorityAndNameQuery("42", "APP/2024/001", UserId: null),
            CancellationToken.None);

        // Assert — only the partitioned method was called; cross-partition scan = 0
        await Assert.That(repository.GetByAuthorityAndNameCallCount).IsEqualTo(1);
        await Assert.That(repository.GetByUidWithoutAuthorityCallCount).IsEqualTo(0);
    }
}
