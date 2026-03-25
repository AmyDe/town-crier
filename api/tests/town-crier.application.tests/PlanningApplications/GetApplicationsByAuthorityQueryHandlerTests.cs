using TownCrier.Application.PlanningApplications;
using TownCrier.Application.Tests.Polling;

namespace TownCrier.Application.Tests.PlanningApplications;

public sealed class GetApplicationsByAuthorityQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnApplications_When_AuthorityHasApplications()
    {
        // Arrange
        var app1 = new PlanningApplicationBuilder()
            .WithUid("uid-001")
            .WithName("APP/2024/001")
            .WithAreaId(42)
            .WithAreaName("Camden")
            .Build();
        var app2 = new PlanningApplicationBuilder()
            .WithUid("uid-002")
            .WithName("APP/2024/002")
            .WithAreaId(42)
            .WithAreaName("Camden")
            .Build();

        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(app1, CancellationToken.None);
        await repository.UpsertAsync(app2, CancellationToken.None);

        var handler = new GetApplicationsByAuthorityQueryHandler(repository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationsByAuthorityQuery(42), CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(2);
    }

    [Test]
    public async Task Should_ReturnEmpty_When_AuthorityHasNoApplications()
    {
        // Arrange
        var repository = new FakePlanningApplicationRepository();
        var handler = new GetApplicationsByAuthorityQueryHandler(repository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationsByAuthorityQuery(999), CancellationToken.None);

        // Assert
        await Assert.That(result).IsEmpty();
    }

    [Test]
    public async Task Should_OnlyReturnApplications_ForRequestedAuthority()
    {
        // Arrange
        var camdenApp = new PlanningApplicationBuilder()
            .WithUid("uid-001")
            .WithName("APP/2024/001")
            .WithAreaId(42)
            .WithAreaName("Camden")
            .Build();
        var islingtonApp = new PlanningApplicationBuilder()
            .WithUid("uid-002")
            .WithName("APP/2024/002")
            .WithAreaId(99)
            .WithAreaName("Islington")
            .Build();

        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(camdenApp, CancellationToken.None);
        await repository.UpsertAsync(islingtonApp, CancellationToken.None);

        var handler = new GetApplicationsByAuthorityQueryHandler(repository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationsByAuthorityQuery(42), CancellationToken.None);

        // Assert
        await Assert.That(result).HasCount().EqualTo(1);
        await Assert.That(result[0].AreaId).IsEqualTo(42);
    }
}
