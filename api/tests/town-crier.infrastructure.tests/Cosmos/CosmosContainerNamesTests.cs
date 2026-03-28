using TownCrier.Infrastructure.Cosmos;

namespace TownCrier.Infrastructure.Tests.Cosmos;

public sealed class CosmosContainerNamesTests
{
    [Test]
    public async Task Should_ExposeDatabaseName_AsConstant()
    {
        await Assert.That(CosmosContainerNames.DatabaseName).IsEqualTo("town-crier");
    }
}
