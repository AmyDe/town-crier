using TownCrier.Application.PlanIt;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Application.Tests.PlanIt;

public sealed class FetchPageResultTests
{
    [Test]
    public async Task Should_ExposeAllConstructorArguments_AsProperties()
    {
        // Arrange
        var applications = new List<PlanningApplication>();

        // Act
        var result = new FetchPageResult(
            PageNumber: 3,
            Applications: applications,
            Total: 7200,
            HasMorePages: true);

        // Assert
        await Assert.That(result.PageNumber).IsEqualTo(3);
        await Assert.That(result.Applications).IsSameReferenceAs(applications);
        await Assert.That(result.Total).IsEqualTo(7200);
        await Assert.That(result.HasMorePages).IsTrue();
    }

    [Test]
    public async Task Should_AllowNullTotal_When_PlanItDoesNotReportIt()
    {
        // Act
        var result = new FetchPageResult(
            PageNumber: 1,
            Applications: [],
            Total: null,
            HasMorePages: false);

        // Assert
        await Assert.That(result.Total).IsNull();
        await Assert.That(result.HasMorePages).IsFalse();
    }
}
