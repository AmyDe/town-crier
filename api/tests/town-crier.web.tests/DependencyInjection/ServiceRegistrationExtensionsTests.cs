using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using TownCrier.Application.DecisionAlerts;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Groups;
using TownCrier.Application.Notifications;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.Polling;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Web.Extensions;

namespace TownCrier.Web.Tests.DependencyInjection;

public sealed class ServiceRegistrationExtensionsTests
{
    [Test]
    public async Task Should_RegisterInfrastructureServices_When_AddInfrastructureServicesCalled()
    {
        // Arrange
        var services = new ServiceCollection();
        var configuration = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["ConnectionStrings:CosmosDb"] = "AccountEndpoint=https://localhost:8081/;AccountKey=C2y6yDjf5/R+ob0N8A7Cgv30VRDJIWEHLM+4QDU5DE2nQ9nDuVTqobD4b8mGGyPMbIZnqyMsEcaGQy67XIw/Jw==",
            })
            .Build();

        // Act
        services.AddInfrastructureServices(configuration);

        // Assert — verify key infrastructure registrations exist
        var provider = services.BuildServiceProvider();
        await Assert.That(provider.GetService<IPlanningApplicationRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IUserProfileRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IWatchZoneRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IGroupRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IGroupInvitationRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IDecisionAlertRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IDeviceRegistrationRepository>()).IsNotNull();
        await Assert.That(provider.GetService<INotificationRepository>()).IsNotNull();
        await Assert.That(provider.GetService<ISavedApplicationRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IPollStateStore>()).IsNotNull();
    }
}
