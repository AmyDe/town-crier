using Microsoft.AspNetCore.Hosting;
using Microsoft.Extensions.DependencyInjection;
using OpenTelemetry;
using OpenTelemetry.Trace;

namespace TownCrier.Web.Tests.Observability;

public sealed class ServiceNameResourceTests
{
    [Test]
    public async Task Should_SetServiceNameToTownCrierApi_When_TracerProviderIsConfigured()
    {
        // Arrange -- the OpenTelemetry resource's service.name attribute is what
        // the Azure Monitor exporter writes to App Insights' cloud_RoleName field
        // (surfaced as AppRoleName in the AppRequests/AppDependencies tables).
        // The API must report itself as "town-crier-api" so its telemetry can be
        // distinguished from the web app's traffic in the shared workspace.
        await using var baseFactory = new TestWebApplicationFactory();
        await using var factory = baseFactory.WithWebHostBuilder(builder =>
        {
            builder.UseSetting(
                "APPLICATIONINSIGHTS_CONNECTION_STRING",
                "InstrumentationKey=00000000-0000-0000-0000-000000000000;IngestionEndpoint=https://localhost/");
        });

        // Force the host to build so OpenTelemetry providers are registered.
        using var client = factory.CreateClient();

        // Act
        var tracerProvider = factory.Services.GetRequiredService<TracerProvider>();
        var resource = tracerProvider.GetResource();

        // Assert
        var serviceNameAttribute = resource.Attributes
            .FirstOrDefault(a => a.Key == "service.name");
        await Assert.That(serviceNameAttribute.Value).IsEqualTo("town-crier-api")
            .Because("the API container must report AppRoleName='town-crier-api' so its telemetry is distinguishable from the web app in App Insights");
    }
}
