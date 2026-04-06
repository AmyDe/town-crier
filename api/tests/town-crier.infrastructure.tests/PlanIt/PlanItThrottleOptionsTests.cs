using Microsoft.Extensions.Configuration;
using TownCrier.Infrastructure.PlanIt;

namespace TownCrier.Infrastructure.Tests.PlanIt;

public sealed class PlanItThrottleOptionsTests
{
    [Test]
    public async Task Should_DefaultToTwoSeconds_When_NoPropertiesSet()
    {
        // Arrange & Act
        var options = new PlanItThrottleOptions();

        // Assert
        await Assert.That(options.DelayBetweenRequestsSeconds).IsEqualTo(2);
        await Assert.That(options.DelayBetweenRequests).IsEqualTo(TimeSpan.FromSeconds(2));
    }

    [Test]
    public async Task Should_ComputeDelayFromSeconds_When_SecondsPropertySet()
    {
        // Arrange & Act
        var options = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 3.5 };

        // Assert
        await Assert.That(options.DelayBetweenRequests).IsEqualTo(TimeSpan.FromSeconds(3.5));
    }

    [Test]
    public async Task Should_ReturnZeroDelay_When_SecondsSetToZero()
    {
        // Arrange & Act
        var options = new PlanItThrottleOptions { DelayBetweenRequestsSeconds = 0 };

        // Assert
        await Assert.That(options.DelayBetweenRequests).IsEqualTo(TimeSpan.Zero);
    }

    [Test]
    public async Task Should_BindFromConfiguration_When_SectionProvided()
    {
        // Arrange
        var config = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["PlanIt:Throttle:DelayBetweenRequestsSeconds"] = "5",
            })
            .Build();

        var options = new PlanItThrottleOptions();
        config.GetSection("PlanIt:Throttle").Bind(options);

        // Assert
        await Assert.That(options.DelayBetweenRequestsSeconds).IsEqualTo(5);
        await Assert.That(options.DelayBetweenRequests).IsEqualTo(TimeSpan.FromSeconds(5));
    }

    [Test]
    public async Task Should_KeepDefaults_When_ConfigSectionEmpty()
    {
        // Arrange
        var config = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>())
            .Build();

        var options = new PlanItThrottleOptions();
        config.GetSection("PlanIt:Throttle").Bind(options);

        // Assert — defaults preserved
        await Assert.That(options.DelayBetweenRequestsSeconds).IsEqualTo(2);
        await Assert.That(options.DelayBetweenRequests).IsEqualTo(TimeSpan.FromSeconds(2));
    }
}
