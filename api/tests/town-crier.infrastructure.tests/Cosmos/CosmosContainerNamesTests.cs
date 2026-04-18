using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Tests.Cosmos;

public sealed class CosmosContainerNamesTests
{
    [Test]
    public async Task Should_ExposeDatabaseName_AsConstant()
    {
        string databaseName = CosmosContainerNames.DatabaseName;
        await Assert.That(databaseName).IsEqualTo("town-crier");
    }

    [Test]
    public async Task Should_ExposeAllContainerNames_AsConstants()
    {
        string users = CosmosContainerNames.Users;
        string notifications = CosmosContainerNames.Notifications;
        string deviceRegistrations = CosmosContainerNames.DeviceRegistrations;
        string decisionAlerts = CosmosContainerNames.DecisionAlerts;
        string savedApplications = CosmosContainerNames.SavedApplications;
        string watchZones = CosmosContainerNames.WatchZones;
        string applications = CosmosContainerNames.Applications;
        string offerCodes = CosmosContainerNames.OfferCodes;

        await Assert.That(users).IsNotEmpty();
        await Assert.That(notifications).IsNotEmpty();
        await Assert.That(deviceRegistrations).IsNotEmpty();
        await Assert.That(decisionAlerts).IsNotEmpty();
        await Assert.That(savedApplications).IsNotEmpty();
        await Assert.That(watchZones).IsNotEmpty();
        await Assert.That(applications).IsNotEmpty();
        await Assert.That(offerCodes).IsEqualTo("OfferCodes");
    }
}
