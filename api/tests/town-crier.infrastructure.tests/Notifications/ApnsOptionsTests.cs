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

    [Test]
    public async Task Should_NotThrow_When_ValidateCalledAndDisabled()
    {
        // Arrange — disabled options with no auth fields. Local-dev mode.
        var options = new ApnsOptions { Enabled = false };

        // Act + Assert
        await Assert.That(options.Validate).ThrowsNothing();
    }

    [Test]
    public async Task Should_NotThrow_When_ValidateCalledAndEnabledWithAllFieldsPopulated()
    {
        // Arrange
        var options = ValidEnabledOptions();

        // Act + Assert
        await Assert.That(options.Validate).ThrowsNothing();
    }

    [Test]
    [Arguments("AuthKey")]
    [Arguments("KeyId")]
    [Arguments("TeamId")]
    [Arguments("BundleId")]
    public async Task Should_Throw_When_EnabledAndRequiredFieldEmpty(string emptyField)
    {
        // Arrange
        var options = ValidEnabledOptions();
        switch (emptyField)
        {
            case "AuthKey": options.AuthKey = string.Empty; break;
            case "KeyId": options.KeyId = string.Empty; break;
            case "TeamId": options.TeamId = string.Empty; break;
            case "BundleId": options.BundleId = string.Empty; break;
        }

        // Act + Assert
        var exception = await Assert.That(options.Validate).Throws<InvalidOperationException>();
        await Assert.That(exception!.Message).Contains(emptyField);
    }

    [Test]
    [Arguments("ABCDEFGHI")] // 9 chars — too short
    [Arguments("ABCDEFGHIJK")] // 11 chars — too long
    [Arguments("")] // empty — required-fields check would also catch this; included for completeness
    public async Task Should_Throw_When_EnabledAndKeyIdNotTenCharacters(string badKeyId)
    {
        // Arrange
        var options = ValidEnabledOptions();
        options.KeyId = badKeyId;

        // Act + Assert
        await Assert.That(options.Validate).Throws<InvalidOperationException>();
    }

    [Test]
    [Arguments("123456789")] // 9 chars — too short
    [Arguments("12345678901")] // 11 chars — too long
    public async Task Should_Throw_When_EnabledAndTeamIdNotTenCharacters(string badTeamId)
    {
        // Arrange
        var options = ValidEnabledOptions();
        options.TeamId = badTeamId;

        // Act + Assert
        var exception = await Assert.That(options.Validate).Throws<InvalidOperationException>();
        await Assert.That(exception!.Message).Contains("TeamId");
    }

    private static ApnsOptions ValidEnabledOptions()
    {
        return new ApnsOptions
        {
            Enabled = true,
            AuthKey = "-----BEGIN PRIVATE KEY-----\nMIG...==\n-----END PRIVATE KEY-----",
            KeyId = "ABCDEFGHIJ",
            TeamId = "1234567890",
            BundleId = "uk.towncrierapp.mobile",
        };
    }
}
