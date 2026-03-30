using System.Reflection;

namespace TownCrier.Infrastructure.Tests.Cosmos;

public sealed class CosmosPackageCleanupTests
{
    [Test]
    public async Task Should_NotReferenceCosmosSDK_When_InfrastructureProjectBuilt()
    {
        // Arrange — the infrastructure assembly should not carry a dependency on the old Cosmos SDK
        var infrastructureAssembly = typeof(TownCrier.Infrastructure.Cosmos.ICosmosRestClient).Assembly;
        var referencedAssemblies = infrastructureAssembly.GetReferencedAssemblies();

        // Act
        var cosmosRef = Array.Find(referencedAssemblies, a => a.Name == "Microsoft.Azure.Cosmos.Client");

        // Assert
        await Assert.That(cosmosRef).IsNull();
    }

    [Test]
    public async Task Should_NotReferenceNewtonsoftJson_When_InfrastructureProjectBuilt()
    {
        // Arrange — the infrastructure assembly should not carry a dependency on Newtonsoft.Json
        var infrastructureAssembly = typeof(TownCrier.Infrastructure.Cosmos.ICosmosRestClient).Assembly;
        var referencedAssemblies = infrastructureAssembly.GetReferencedAssemblies();

        // Act
        var newtonsoftRef = Array.Find(referencedAssemblies, a => a.Name == "Newtonsoft.Json");

        // Assert
        await Assert.That(newtonsoftRef).IsNull();
    }
}
