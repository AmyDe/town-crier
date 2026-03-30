using TownCrier.Domain.DeviceRegistrations;
using TownCrier.Infrastructure.DeviceRegistrations;
using TownCrier.Infrastructure.Tests.Cosmos;

namespace TownCrier.Infrastructure.Tests.DeviceRegistrations;

public sealed class CosmosDeviceRegistrationRepositoryTests
{
    [Test]
    public async Task Should_PersistRegistration_When_SaveCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosDeviceRegistrationRepository(client);
        var reg = DeviceRegistration.Create("user-1", "token-abc", DevicePlatform.Ios, DateTimeOffset.UtcNow);

        // Act
        await repo.SaveAsync(reg, CancellationToken.None);

        // Assert
        var result = await repo.GetByUserIdAsync("user-1", CancellationToken.None);
        await Assert.That(result.Count).IsEqualTo(1);
        await Assert.That(result[0].Token).IsEqualTo("token-abc");
    }

    [Test]
    public async Task Should_ReturnRegistration_When_GetByTokenCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosDeviceRegistrationRepository(client);
        var reg = DeviceRegistration.Create("user-1", "token-abc", DevicePlatform.Ios, DateTimeOffset.UtcNow);
        await repo.SaveAsync(reg, CancellationToken.None);

        // Act — fake returns all in collection, no SQL filtering
        var result = await repo.GetByTokenAsync("token-abc", CancellationToken.None);

        // Assert
        await Assert.That(result).IsNotNull();
        await Assert.That(result!.Token).IsEqualTo("token-abc");
    }

    [Test]
    public async Task Should_ReturnNull_When_GetByTokenForMissingToken()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosDeviceRegistrationRepository(client);

        // Act
        var result = await repo.GetByTokenAsync("nonexistent", CancellationToken.None);

        // Assert
        await Assert.That(result).IsNull();
    }

    [Test]
    public async Task Should_DeleteRegistration_When_DeleteByTokenCalled()
    {
        // Arrange
        var client = new FakeCosmosRestClient();
        var repo = new CosmosDeviceRegistrationRepository(client);
        var reg = DeviceRegistration.Create("user-1", "token-abc", DevicePlatform.Ios, DateTimeOffset.UtcNow);
        await repo.SaveAsync(reg, CancellationToken.None);

        // Act
        await repo.DeleteByTokenAsync("token-abc", CancellationToken.None);

        // Assert
        var result = await repo.GetByUserIdAsync("user-1", CancellationToken.None);
        await Assert.That(result.Count).IsEqualTo(0);
    }
}
