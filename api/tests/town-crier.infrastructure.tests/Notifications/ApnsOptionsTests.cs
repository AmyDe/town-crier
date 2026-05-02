using Microsoft.Extensions.Configuration;
using TownCrier.Infrastructure.Notifications;

namespace TownCrier.Infrastructure.Tests.Notifications;

public sealed class ApnsOptionsTests
{
    [Test]
    public async Task Should_BindAllPropertiesFromApnsSection_When_LoadFromConfigurationCalled()
    {
        // Arrange
        var configuration = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["Apns:Enabled"] = "true",
                ["Apns:AuthKey"] = "-----BEGIN PRIVATE KEY-----\nABC\n-----END PRIVATE KEY-----",
                ["Apns:KeyId"] = "ABCDEFGHIJ",
                ["Apns:TeamId"] = "1234567890",
                ["Apns:BundleId"] = "uk.towncrierapp.mobile",
                ["Apns:UseSandbox"] = "true",
                ["Apns:MaxParallelism"] = "5",
            })
            .Build();

        // Act
        var options = ApnsOptions.LoadFromConfiguration(configuration);

        // Assert
        await Assert.That(options.Enabled).IsTrue();
        await Assert.That(options.AuthKey).IsEqualTo("-----BEGIN PRIVATE KEY-----\nABC\n-----END PRIVATE KEY-----");
        await Assert.That(options.KeyId).IsEqualTo("ABCDEFGHIJ");
        await Assert.That(options.TeamId).IsEqualTo("1234567890");
        await Assert.That(options.BundleId).IsEqualTo("uk.towncrierapp.mobile");
        await Assert.That(options.UseSandbox).IsTrue();
        await Assert.That(options.MaxParallelism).IsEqualTo(5);
    }

    [Test]
    public async Task Should_DefaultToDisabled_When_ApnsSectionMissing()
    {
        // Arrange
        var configuration = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>())
            .Build();

        // Act
        var options = ApnsOptions.LoadFromConfiguration(configuration);

        // Assert
        await Assert.That(options.Enabled).IsFalse();
        await Assert.That(options.AuthKey).IsEqualTo(string.Empty);
        await Assert.That(options.KeyId).IsEqualTo(string.Empty);
        await Assert.That(options.TeamId).IsEqualTo(string.Empty);
    }
}
