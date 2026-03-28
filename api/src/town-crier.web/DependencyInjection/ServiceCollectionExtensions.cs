using Microsoft.AspNetCore.Authentication.JwtBearer;
using TownCrier.Application.Authorities;
using TownCrier.Application.DecisionAlerts;
using TownCrier.Application.DemoAccount;
using TownCrier.Application.Designations;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Geocoding;
using TownCrier.Application.Groups;
using TownCrier.Application.Notifications;
using TownCrier.Application.PlanIt;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.Polling;
using TownCrier.Application.RateLimiting;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.Search;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.Polling;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.DecisionAlerts;
using TownCrier.Infrastructure.DeviceRegistrations;
using TownCrier.Infrastructure.Geocoding;
using TownCrier.Infrastructure.GovUkPlanningData;
using TownCrier.Infrastructure.Groups;
using TownCrier.Infrastructure.Notifications;
using TownCrier.Infrastructure.PlanIt;
using TownCrier.Infrastructure.PlanningApplications;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.RateLimiting;
using TownCrier.Infrastructure.SavedApplications;
using TownCrier.Infrastructure.UserProfiles;
using TownCrier.Infrastructure.WatchZones;
using TownCrier.Web.Polling;
using TownCrier.Web.RateLimiting;

namespace TownCrier.Web.DependencyInjection;

internal static class ServiceCollectionExtensions
{
    public static IServiceCollection AddApplicationServices(this IServiceCollection services)
    {
        // Geocoding
        services.AddTransient<GeocodePostcodeQueryHandler>();

        // Authorities
        services.AddTransient<GetAuthoritiesQueryHandler>();
        services.AddTransient<GetAuthorityByIdQueryHandler>();

        // Designations
        services.AddTransient<GetDesignationContextQueryHandler>();

        // User profiles
        services.AddTransient<CreateUserProfileCommandHandler>();
        services.AddTransient<GetUserProfileQueryHandler>();
        services.AddTransient<UpdateUserProfileCommandHandler>();
        services.AddTransient<ExportUserDataQueryHandler>();
        services.AddTransient<DeleteUserProfileCommandHandler>();

        // Watch zones
        services.AddTransient<UpdateZonePreferencesCommandHandler>();
        services.AddTransient<GetZonePreferencesQueryHandler>();
        services.AddTransient<CreateWatchZoneCommandHandler>();
        services.AddTransient<ListWatchZonesQueryHandler>();
        services.AddTransient<DeleteWatchZoneCommandHandler>();

        // Device registrations
        services.AddTransient<RegisterDeviceTokenCommandHandler>();
        services.AddTransient<RemoveInvalidDeviceTokenCommandHandler>();

        // Planning applications
        services.AddTransient<GetApplicationByUidQueryHandler>();
        services.AddTransient<GetApplicationsByAuthorityQueryHandler>();
        services.AddTransient<SearchPlanningApplicationsQueryHandler>();

        // Notifications
        services.AddTransient<GetNotificationsQueryHandler>();

        // Saved applications
        services.AddTransient<SaveApplicationCommandHandler>();
        services.AddTransient<RemoveSavedApplicationCommandHandler>();
        services.AddTransient<GetSavedApplicationsQueryHandler>();

        // Demo account
        services.AddTransient<GetDemoAccountQueryHandler>();

        // Groups
        services.AddTransient<CreateGroupCommandHandler>();
        services.AddTransient<GetGroupQueryHandler>();
        services.AddTransient<GetUserGroupsQueryHandler>();
        services.AddTransient<InviteMemberCommandHandler>();
        services.AddTransient<AcceptInvitationCommandHandler>();
        services.AddTransient<RemoveGroupMemberCommandHandler>();
        services.AddTransient<DeleteGroupCommandHandler>();

        // Polling
        services.AddTransient<PollPlanItCommandHandler>();

        return services;
    }

    public static IServiceCollection AddInfrastructureServices(
        this IServiceCollection services,
        IConfiguration configuration)
    {
        // Cosmos DB
        services.AddCosmosClient(configuration);

        // Postcodes.io geocoding & authority resolver
#pragma warning disable S1075 // Hardcoded URI is a sensible default
        var postcodesIoBaseUrl = configuration["PostcodesIo:BaseUrl"] ?? "https://api.postcodes.io/";
#pragma warning restore S1075
        services.AddHttpClient<IPostcodeGeocoder, PostcodesIoGeocoder>(client =>
        {
            client.BaseAddress = new Uri(postcodesIoBaseUrl);
        });
        services.AddSingleton<IAuthorityResolver>(sp =>
        {
            var factory = sp.GetRequiredService<IHttpClientFactory>();
            var httpClient = factory.CreateClient("PostcodesIoResolver");
            httpClient.BaseAddress = new Uri(postcodesIoBaseUrl);
            return new PostcodesIoAuthorityResolver(httpClient, sp.GetRequiredService<IAuthorityProvider>());
        });

        // PlanIt client & authority provider
#pragma warning disable S1075 // Hardcoded URI is a sensible default
        var planItBaseUrl = configuration["PlanIt:BaseUrl"] ?? "https://www.planit.org.uk/";
#pragma warning restore S1075
        services.AddHttpClient<IPlanItClient, PlanItClient>(client =>
        {
            client.BaseAddress = new Uri(planItBaseUrl);
        });
        services.AddSingleton<IAuthorityProvider>(sp =>
        {
            var factory = sp.GetRequiredService<IHttpClientFactory>();
            var httpClient = factory.CreateClient("PlanItAreas");
            httpClient.BaseAddress = new Uri(planItBaseUrl);
            return new CachedPlanItAuthorityProvider(httpClient, sp.GetRequiredService<TimeProvider>());
        });

        // Gov.UK Planning Data (designations)
#pragma warning disable S1075 // Hardcoded URI is a sensible default
        var govUkBaseUrl = configuration["GovUkPlanningData:BaseUrl"] ?? "https://www.planning.data.gov.uk/";
#pragma warning restore S1075
        services.AddHttpClient<IDesignationDataProvider, GovUkPlanningDataClient>(client =>
        {
            client.BaseAddress = new Uri(govUkBaseUrl);
        });

        // Repositories
        services.AddSingleton<IDecisionAlertRepository, CosmosDecisionAlertRepository>();
        services.AddSingleton<IPlanningApplicationRepository, CosmosPlanningApplicationRepository>();
        services.AddSingleton<IUserProfileRepository, CosmosUserProfileRepository>();
        services.AddSingleton<IWatchZoneRepository, CosmosWatchZoneRepository>();
        services.AddSingleton<ISavedApplicationRepository, CosmosSavedApplicationRepository>();
        services.AddSingleton<IGroupRepository, CosmosGroupRepository>();
        services.AddSingleton<IGroupInvitationRepository, CosmosGroupInvitationRepository>();
        services.AddSingleton<IDeviceRegistrationRepository>(sp =>
            new CosmosDeviceRegistrationRepository(sp.GetRequiredService<Microsoft.Azure.Cosmos.CosmosClient>()));
        services.AddSingleton<INotificationRepository>(sp =>
            new CosmosNotificationRepository(sp.GetRequiredService<Microsoft.Azure.Cosmos.CosmosClient>()));

        // Polling infrastructure
        var pollStateFilePath = configuration["Polling:StateFilePath"]
            ?? Path.Combine(AppContext.BaseDirectory, "poll-state.txt");
        services.AddSingleton<IPollStateStore>(new FilePollStateStore(pollStateFilePath));
        services.AddSingleton<IActiveAuthorityProvider, InMemoryActiveAuthorityProvider>();
        services.AddSingleton<IPollingHealthStore, InMemoryPollingHealthStore>();
        services.AddSingleton<IPollingHealthAlerter, LogPollingHealthAlerter>();
        services.AddSingleton(new PollingHealthConfig(
            StalenessThreshold: TimeSpan.FromHours(1),
            MaxConsecutiveFailures: 5));
        services.AddSingleton(TimeProvider.System);
        services.AddSingleton<INotificationEnqueuer, LogNotificationEnqueuer>();
        services.AddSingleton(new PollingScheduleConfig(
            HighThreshold: configuration.GetValue("Polling:HighThreshold", 5),
            LowThreshold: configuration.GetValue("Polling:LowThreshold", 2)));
        services.AddHostedService<PlanItPollingService>();

        // Rate limiting
        services.AddSingleton<IRateLimitStore, InMemoryRateLimitStore>();

        return services;
    }

    public static IServiceCollection AddAuthenticationServices(
        this IServiceCollection services,
        IConfiguration configuration)
    {
        services.AddAuthentication(JwtBearerDefaults.AuthenticationScheme)
            .AddJwtBearer(options =>
            {
                var domain = configuration["Auth0:Domain"]
                    ?? throw new InvalidOperationException("Auth0:Domain configuration is required.");
                var audience = configuration["Auth0:Audience"]
                    ?? throw new InvalidOperationException("Auth0:Audience configuration is required.");

                options.Authority = $"https://{domain}/";
                options.Audience = audience;
            });

        services.AddAuthorizationBuilder()
            .AddFallbackPolicy("Authenticated", policy => policy.RequireAuthenticatedUser());

        return services;
    }
}
