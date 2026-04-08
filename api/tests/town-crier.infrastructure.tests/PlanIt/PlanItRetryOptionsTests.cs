using Microsoft.Extensions.Configuration;
using TownCrier.Infrastructure.PlanIt;

namespace TownCrier.Infrastructure.Tests.PlanIt;

public sealed class PlanItRetryOptionsTests
{
    [Test]
    public async Task Should_DefaultToOneSecondBaseDelay_When_NoPropertiesSet()
    {
        // Arrange & Act
        var options = new PlanItRetryOptions();

        // Assert
        await Assert.That(options.BaseDelaySeconds).IsEqualTo(1);
        await Assert.That(options.BaseDelay).IsEqualTo(TimeSpan.FromSeconds(1));
        await Assert.That(options.MaxRetries).IsEqualTo(5);
    }

    [Test]
    public async Task Should_ComputeBaseDelayFromSeconds_When_SecondsPropertySet()
    {
        // Arrange & Act
        var options = new PlanItRetryOptions { BaseDelaySeconds = 2.5 };

        // Assert
        await Assert.That(options.BaseDelay).IsEqualTo(TimeSpan.FromSeconds(2.5));
    }

    [Test]
    public async Task Should_AllowCustomMaxRetries_When_Set()
    {
        // Arrange & Act
        var options = new PlanItRetryOptions { MaxRetries = 10, BaseDelaySeconds = 3 };

        // Assert
        await Assert.That(options.MaxRetries).IsEqualTo(10);
        await Assert.That(options.BaseDelay).IsEqualTo(TimeSpan.FromSeconds(3));
    }

    [Test]
    public async Task Should_BindFromConfiguration_When_SectionProvided()
    {
        // Arrange
        var config = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["PlanIt:Retry:MaxRetries"] = "10",
                ["PlanIt:Retry:BaseDelaySeconds"] = "3",
            })
            .Build();

        var options = new PlanItRetryOptions();
        config.GetSection("PlanIt:Retry").Bind(options);

        // Assert
        await Assert.That(options.MaxRetries).IsEqualTo(10);
        await Assert.That(options.BaseDelaySeconds).IsEqualTo(3);
        await Assert.That(options.BaseDelay).IsEqualTo(TimeSpan.FromSeconds(3));
    }

    [Test]
    public async Task Should_KeepDefaults_When_ConfigSectionEmpty()
    {
        // Arrange
        var config = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>())
            .Build();

        var options = new PlanItRetryOptions();
        config.GetSection("PlanIt:Retry").Bind(options);

        // Assert — defaults preserved
        await Assert.That(options.MaxRetries).IsEqualTo(5);
        await Assert.That(options.BaseDelaySeconds).IsEqualTo(1);
        await Assert.That(options.BaseDelay).IsEqualTo(TimeSpan.FromSeconds(1));
    }

    [Test]
    public async Task Should_DefaultMaxRetryAfterTo30Seconds_When_NoPropertiesSet()
    {
        // Arrange & Act
        var options = new PlanItRetryOptions();

        // Assert
        await Assert.That(options.MaxRetryAfterSeconds).IsEqualTo(30);
        await Assert.That(options.MaxRetryAfter).IsEqualTo(TimeSpan.FromSeconds(30));
    }

    [Test]
    public async Task Should_AllowCustomMaxRetryAfter_When_Set()
    {
        // Arrange & Act
        var options = new PlanItRetryOptions { MaxRetryAfterSeconds = 60 };

        // Assert
        await Assert.That(options.MaxRetryAfterSeconds).IsEqualTo(60);
        await Assert.That(options.MaxRetryAfter).IsEqualTo(TimeSpan.FromSeconds(60));
    }

    [Test]
    public async Task Should_BindMaxRetryAfterFromConfiguration_When_SectionProvided()
    {
        // Arrange
        var config = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["PlanIt:Retry:MaxRetryAfterSeconds"] = "45",
            })
            .Build();

        var options = new PlanItRetryOptions();
        config.GetSection("PlanIt:Retry").Bind(options);

        // Assert
        await Assert.That(options.MaxRetryAfterSeconds).IsEqualTo(45);
        await Assert.That(options.MaxRetryAfter).IsEqualTo(TimeSpan.FromSeconds(45));
    }
}
