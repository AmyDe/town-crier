using System.Security.Cryptography;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.Extensions.Logging;
using TownCrier.Application.Notifications;
using TownCrier.Infrastructure.Notifications;

namespace TownCrier.Infrastructure.Tests.Notifications;

public sealed class ApnsServiceCollectionExtensionsTests
{
    [Test]
    public async Task Should_RegisterNoOpSender_When_ApnsDisabled()
    {
        // Arrange
        var configuration = BuildConfiguration(("Apns:Enabled", "false"));
        var services = BaseServices();

        // Act
        services.AddApnsPushNotifications(configuration);

        // Assert
        await using var provider = services.BuildServiceProvider();
        var sender = provider.GetRequiredService<IPushNotificationSender>();
        await Assert.That(sender).IsTypeOf<NoOpPushNotificationSender>();
    }

    [Test]
    public async Task Should_RegisterNoOpSender_When_ApnsSectionMissing()
    {
        // Arrange — neither Apns:Enabled nor any other Apns key is present.
        var configuration = BuildConfiguration();
        var services = BaseServices();

        // Act
        services.AddApnsPushNotifications(configuration);

        // Assert
        await using var provider = services.BuildServiceProvider();
        var sender = provider.GetRequiredService<IPushNotificationSender>();
        await Assert.That(sender).IsTypeOf<NoOpPushNotificationSender>();
    }

    [Test]
    public async Task Should_RegisterApnsSender_When_ApnsEnabledWithValidConfig()
    {
        // Arrange
        var configuration = BuildEnabledConfiguration();
        var services = BaseServices();

        // Act
        services.AddApnsPushNotifications(configuration);

        // Assert
        await using var provider = services.BuildServiceProvider();
        var sender = provider.GetRequiredService<IPushNotificationSender>();
        await Assert.That(sender).IsTypeOf<ApnsPushNotificationSender>();
    }

    [Test]
    public async Task Should_RegisterJwtProviderAsSingleton_When_ApnsEnabled()
    {
        // Arrange
        var configuration = BuildEnabledConfiguration();
        var services = BaseServices();
        services.AddApnsPushNotifications(configuration);

        // Act
        await using var provider = services.BuildServiceProvider();
        var first = provider.GetRequiredService<ApnsJwtProvider>();
        var second = provider.GetRequiredService<ApnsJwtProvider>();

        // Assert — singleton means the same instance is reused.
        await Assert.That(ReferenceEquals(first, second)).IsTrue();
    }

    [Test]
    public async Task Should_RegisterApnsOptionsAsSingleton_When_ApnsEnabled()
    {
        // Arrange
        var configuration = BuildEnabledConfiguration();
        var services = BaseServices();
        services.AddApnsPushNotifications(configuration);

        // Act
        await using var provider = services.BuildServiceProvider();
        var options = provider.GetRequiredService<ApnsOptions>();

        // Assert
        await Assert.That(options.Enabled).IsTrue();
        await Assert.That(options.BundleId).IsEqualTo("uk.towncrierapp.mobile");
        await Assert.That(options.UseSandbox).IsTrue();
    }

    [Test]
    public async Task Should_ThrowAtRegistration_When_ApnsEnabledButRequiredFieldMissing()
    {
        // Arrange — Enabled=true but AuthKey is missing. Validation runs at startup.
        var configuration = BuildConfiguration(
            ("Apns:Enabled", "true"),
            ("Apns:KeyId", "ABCDEFGHIJ"),
            ("Apns:TeamId", "1234567890"),
            ("Apns:BundleId", "uk.towncrierapp.mobile"),
            ("Apns:UseSandbox", "true"));
        var services = BaseServices();

        // Act + Assert
        var ex = Assert.Throws<InvalidOperationException>(() =>
            services.AddApnsPushNotifications(configuration));
        await Assert.That(ex!.Message).Contains("AuthKey");
    }

    [Test]
    public async Task Should_ThrowAtRegistration_When_ApnsEnabledButKeyIdWrongLength()
    {
        // Arrange — Enabled=true, KeyId is 9 chars instead of 10.
        var configuration = BuildConfiguration(
            ("Apns:Enabled", "true"),
            ("Apns:AuthKey", GenerateTestPem()),
            ("Apns:KeyId", "ABCDEFGHI"),
            ("Apns:TeamId", "1234567890"),
            ("Apns:BundleId", "uk.towncrierapp.mobile"),
            ("Apns:UseSandbox", "true"));
        var services = BaseServices();

        // Act + Assert
        var ex = Assert.Throws<InvalidOperationException>(() =>
            services.AddApnsPushNotifications(configuration));
        await Assert.That(ex!.Message).Contains("KeyId");
    }

    private static ServiceCollection BaseServices()
    {
        var services = new ServiceCollection();
        services.AddSingleton(TimeProvider.System);
        services.AddLogging(); // ApnsPushNotificationSender takes ILogger<T>.
        return services;
    }

    private static IConfiguration BuildConfiguration(params (string Key, string Value)[] entries)
    {
        var dict = entries.ToDictionary(e => e.Key, e => (string?)e.Value);
        return new ConfigurationBuilder().AddInMemoryCollection(dict).Build();
    }

    private static IConfiguration BuildEnabledConfiguration()
    {
        return BuildConfiguration(
            ("Apns:Enabled", "true"),
            ("Apns:AuthKey", GenerateTestPem()),
            ("Apns:KeyId", "ABCDEFGHIJ"),
            ("Apns:TeamId", "1234567890"),
            ("Apns:BundleId", "uk.towncrierapp.mobile"),
            ("Apns:UseSandbox", "true"));
    }

    private static string GenerateTestPem()
    {
        // ApnsJwtProvider.ImportFromPem requires a valid PKCS#8 EC key. Generate
        // an ephemeral one per test invocation — no real keys in source.
        using var ecdsa = ECDsa.Create(ECCurve.NamedCurves.nistP256);
        return ecdsa.ExportPkcs8PrivateKeyPem();
    }
}
