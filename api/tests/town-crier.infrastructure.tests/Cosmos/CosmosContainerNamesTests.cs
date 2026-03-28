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
}
