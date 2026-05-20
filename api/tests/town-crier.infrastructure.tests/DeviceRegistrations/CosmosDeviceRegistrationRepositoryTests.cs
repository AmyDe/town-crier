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

        // Act — partitioned point read: (userId="user-1", token="token-abc")
        var result = await repo.GetByTokenAsync("user-1", "token-abc", CancellationToken.None);

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

        // Act — partitioned point read returns null when no document exists
        var result = await repo.GetByTokenAsync("user-1", "nonexistent", CancellationToken.None);

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

        // Act — partitioned delete: scoped to user-1's partition, no cross-partition scan
        await repo.DeleteByTokenAsync("user-1", "token-abc", CancellationToken.None);

        // Assert
        var result = await repo.GetByUserIdAsync("user-1", CancellationToken.None);
        await Assert.That(result.Count).IsEqualTo(0);
    }

    [Test]
    public async Task Should_NotDeleteOtherUsersToken_When_DeleteByTokenCalledForDifferentUser()
    {
        // Arrange — user-2's token has the same value but is in a different partition
        var client = new FakeCosmosRestClient();
        var repo = new CosmosDeviceRegistrationRepository(client);
        var regUser1 = DeviceRegistration.Create("user-1", "token-abc", DevicePlatform.Ios, DateTimeOffset.UtcNow);
        var regUser2 = DeviceRegistration.Create("user-2", "token-abc", DevicePlatform.Ios, DateTimeOffset.UtcNow);
        await repo.SaveAsync(regUser1, CancellationToken.None);
        await repo.SaveAsync(regUser2, CancellationToken.None);

        // Act — delete only from user-1's partition
        await repo.DeleteByTokenAsync("user-1", "token-abc", CancellationToken.None);

        // Assert — user-2's registration is unaffected
        var user2Regs = await repo.GetByUserIdAsync("user-2", CancellationToken.None);
        await Assert.That(user2Regs.Count).IsEqualTo(1);
        await Assert.That(user2Regs[0].Token).IsEqualTo("token-abc");
    }
}
