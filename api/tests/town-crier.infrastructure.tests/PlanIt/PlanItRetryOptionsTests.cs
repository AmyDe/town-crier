using TownCrier.Infrastructure.PlanIt;

namespace TownCrier.Infrastructure.Tests.PlanIt;

public sealed class PlanItRetryOptionsTests
{
    [Test]
    public async Task Should_DefaultToThreeRetries_When_NoPropertiesSet()
    {
        // Arrange & Act
        var options = new PlanItRetryOptions();

        // Assert
        await Assert.That(options.MaxRetries).IsEqualTo(3);
    }

    [Test]
    public async Task Should_DefaultToOneSecondInitialBackoff_When_NoPropertiesSet()
    {
        // Arrange & Act
        var options = new PlanItRetryOptions();

        // Assert
        await Assert.That(options.InitialBackoffSeconds).IsEqualTo(1);
        await Assert.That(options.InitialBackoff).IsEqualTo(TimeSpan.FromSeconds(1));
    }

    [Test]
    public async Task Should_DefaultToFiveSecondRateLimitBackoff_When_NoPropertiesSet()
    {
        // Arrange & Act
        var options = new PlanItRetryOptions();

        // Assert
        await Assert.That(options.RateLimitBackoffSeconds).IsEqualTo(5);
        await Assert.That(options.RateLimitBackoff).IsEqualTo(TimeSpan.FromSeconds(5));
    }

    [Test]
    public async Task Should_ComputeBackoffFromSeconds_When_SecondsPropertySet()
    {
        // Arrange & Act
        var options = new PlanItRetryOptions
        {
            InitialBackoffSeconds = 2.5,
            RateLimitBackoffSeconds = 10,
        };

        // Assert
        await Assert.That(options.InitialBackoff).IsEqualTo(TimeSpan.FromSeconds(2.5));
        await Assert.That(options.RateLimitBackoff).IsEqualTo(TimeSpan.FromSeconds(10));
    }
}
