using Microsoft.AspNetCore.Authentication;
using Microsoft.Extensions.Configuration;
using Microsoft.Extensions.DependencyInjection;
using TownCrier.Application.Authorities;
using TownCrier.Application.DecisionAlerts;
using TownCrier.Application.DemoAccount;
using TownCrier.Application.Designations;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Geocoding;
using TownCrier.Application.Notifications;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.Search;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Web.Extensions;

namespace TownCrier.Web.Tests.DependencyInjection;

public sealed class ServiceRegistrationExtensionsTests
{
    private static readonly Dictionary<string, string?> CosmosRestConfig = new()
    {
        ["Cosmos:AccountEndpoint"] = "https://test-account.documents.azure.com:443",
        ["Cosmos:DatabaseName"] = "town-crier",
    };

    [Test]
    public async Task Should_RegisterInfrastructureServices_When_AddInfrastructureServicesCalled()
    {
        // Arrange
        var services = new ServiceCollection();
        var configuration = new ConfigurationBuilder()
            .AddInMemoryCollection(CosmosRestConfig)
            .Build();

        // Act
        services.AddInfrastructureServices(configuration);

        // Assert — verify key infrastructure registrations exist
        var provider = services.BuildServiceProvider();
        await Assert.That(provider.GetService<ICosmosRestClient>()).IsNotNull();
        await Assert.That(provider.GetService<IPlanningApplicationRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IUserProfileRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IWatchZoneRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IDecisionAlertRepository>()).IsNotNull();
        await Assert.That(provider.GetService<IDeviceRegistrationRepository>()).IsNotNull();
        await Assert.That(provider.GetService<INotificationRepository>()).IsNotNull();
        await Assert.That(provider.GetService<ISavedApplicationRepository>()).IsNotNull();
    }

    [Test]
    public async Task Should_RegisterApplicationHandlers_When_AddApplicationServicesCalled()
    {
        // Arrange
        var services = new ServiceCollection();
        var configuration = new ConfigurationBuilder()
            .AddInMemoryCollection(CosmosRestConfig)
            .Build();

        // Infrastructure services needed as dependencies for handlers
        services.AddInfrastructureServices(configuration);

        // Act
        services.AddApplicationServices(configuration);

        // Assert — verify key handler registrations exist
        var provider = services.BuildServiceProvider();
        await Assert.That(provider.GetService<GeocodePostcodeQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetAuthoritiesQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetAuthorityByIdQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetDesignationContextQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<CreateUserProfileCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetUserProfileQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<UpdateUserProfileCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<DeleteUserProfileCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<CreateWatchZoneCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<ListWatchZonesQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<DeleteWatchZoneCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<RegisterDeviceTokenCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetApplicationByUidQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<SearchPlanningApplicationsQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetNotificationsQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<SaveApplicationCommandHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetSavedApplicationsQueryHandler>()).IsNotNull();
        await Assert.That(provider.GetService<GetDemoAccountQueryHandler>()).IsNotNull();
    }

    [Test]
    public async Task Should_ConfigureAuthentication_When_AddAuthenticationServicesCalled()
    {
        // Arrange
        var services = new ServiceCollection();
        var configuration = new ConfigurationBuilder()
            .AddInMemoryCollection(new Dictionary<string, string?>
            {
                ["Auth0:Domain"] = "test.auth0.com",
                ["Auth0:Audience"] = "https://api.towncrier.app",
            })
            .Build();

        // Act
        services.AddAuthenticationServices(configuration);

        // Assert — verify authentication and authorization are registered
        var provider = services.BuildServiceProvider();
        await Assert.That(provider.GetService<IAuthenticationService>()).IsNotNull();
    }
}
