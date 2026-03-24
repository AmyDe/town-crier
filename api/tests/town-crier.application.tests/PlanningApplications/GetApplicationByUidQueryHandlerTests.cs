using TownCrier.Application.PlanningApplications;
using TownCrier.Application.Tests.Polling;

namespace TownCrier.Application.Tests.PlanningApplications;

public sealed class GetApplicationByUidQueryHandlerTests
{
    [Test]
    public async Task Should_ReturnApplication_When_FoundByUid()
    {
        // Arrange
        var application = new PlanningApplicationBuilder()
            .WithUid("planit-uid-001")
            .WithName("APP/2024/001")
            .WithAreaId(42)
            .WithAreaName("Camden")
            .Build();

        var repository = new FakePlanningApplicationRepository();
        await repository.UpsertAsync(application, CancellationToken.None);

        var handler = new GetApplicationByUidQueryHandler(repository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByUidQuery("planit-uid-001"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Uid).IsEqualTo("planit-uid-001");
        await Assert.That(result.Name).IsEqualTo("APP/2024/001");
        await Assert.That(result.AreaId).IsEqualTo(42);
        await Assert.That(result.AreaName).IsEqualTo("Camden");
    }

    [Test]
    public async Task Should_ReturnNull_When_UidNotFound()
    {
        // Arrange
        var repository = new FakePlanningApplicationRepository();
        var handler = new GetApplicationByUidQueryHandler(repository);

        // Act
        var result = await handler.HandleAsync(
            new GetApplicationByUidQuery("nonexistent-uid"), CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }
}
