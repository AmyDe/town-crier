using Microsoft.AspNetCore.Authentication.JwtBearer;
using TownCrier.Application.Admin;
using TownCrier.Application.Authorities;
using TownCrier.Application.DecisionAlerts;
using TownCrier.Application.DemoAccount;
using TownCrier.Application.Designations;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Geocoding;
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
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.DecisionAlerts;
using TownCrier.Infrastructure.DeviceRegistrations;
using TownCrier.Infrastructure.Geocoding;
using TownCrier.Infrastructure.GovUkPlanningData;
using TownCrier.Infrastructure.Notifications;
using TownCrier.Infrastructure.PlanIt;
using TownCrier.Infrastructure.PlanningApplications;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.RateLimiting;
using TownCrier.Infrastructure.SavedApplications;
using TownCrier.Infrastructure.UserProfiles;
using TownCrier.Infrastructure.WatchZones;

namespace TownCrier.Web.Extensions;

internal static class ServiceCollectionExtensions
{
    public static IServiceCollection AddInfrastructureServices(
        this IServiceCollection services, IConfiguration configuration)
    {
        services.AddCosmosRestClient(configuration);

        services.AddSingleton<IDecisionAlertRepository, CosmosDecisionAlertRepository>();
        services.AddSingleton<IPlanningApplicationRepository, CosmosPlanningApplicationRepository>();
        services.AddSingleton<IUserProfileRepository, CosmosUserProfileRepository>();
        services.AddSingleton<IWatchZoneRepository, CosmosWatchZoneRepository>();
        services.AddSingleton<ISavedApplicationRepository, CosmosSavedApplicationRepository>();
        services.AddSingleton<IDeviceRegistrationRepository, CosmosDeviceRegistrationRepository>();
        services.AddSingleton<INotificationRepository, CosmosNotificationRepository>();

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

        services.AddSingleton<IRateLimitStore, InMemoryRateLimitStore>();
        services.Configure<RateLimitOptions>(configuration.GetSection("RateLimiting"));

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
            return new PostcodesIoAuthorityResolver(httpClient);
        });

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

#pragma warning disable S1075 // Hardcoded URI is a sensible default
        var govUkBaseUrl = configuration["GovUkPlanningData:BaseUrl"] ?? "https://www.planning.data.gov.uk/";
#pragma warning restore S1075
        services.AddHttpClient<IDesignationDataProvider, GovUkPlanningDataClient>(client =>
        {
            client.BaseAddress = new Uri(govUkBaseUrl);
        });

        return services;
    }

    public static IServiceCollection AddApplicationServices(
        this IServiceCollection services, IConfiguration configuration)
    {
        services.AddTransient<GeocodePostcodeQueryHandler>();
        services.AddTransient<GetAuthoritiesQueryHandler>();
        services.AddTransient<GetAuthorityByIdQueryHandler>();
        services.AddTransient<GetDesignationContextQueryHandler>();

        services.AddTransient<PollPlanItCommandHandler>();

        services.AddSingleton(new AutoGrantOptions
        {
            ProDomains = configuration["Subscription:AutoGrant:ProDomains"] ?? string.Empty,
        });
        services.AddTransient<CreateUserProfileCommandHandler>();
        services.AddTransient<GetUserProfileQueryHandler>();
        services.AddTransient<UpdateUserProfileCommandHandler>();
        services.AddTransient<ExportUserDataQueryHandler>();
        services.AddTransient<DeleteUserProfileCommandHandler>();
        services.AddTransient<UpdateZonePreferencesCommandHandler>();
        services.AddTransient<GetZonePreferencesQueryHandler>();

        services.AddTransient<CreateWatchZoneCommandHandler>();
        services.AddTransient<ListWatchZonesQueryHandler>();
        services.AddTransient<DeleteWatchZoneCommandHandler>();

        services.AddTransient<RegisterDeviceTokenCommandHandler>();
        services.AddTransient<RemoveInvalidDeviceTokenCommandHandler>();

        services.AddTransient<GetApplicationByUidQueryHandler>();
        services.AddTransient<GetApplicationsByAuthorityQueryHandler>();
        services.AddTransient<GetUserApplicationAuthoritiesQueryHandler>();
        services.AddTransient<SearchPlanningApplicationsQueryHandler>();

        services.AddTransient<GetNotificationsQueryHandler>();

        services.AddTransient<SaveApplicationCommandHandler>();
        services.AddTransient<RemoveSavedApplicationCommandHandler>();
        services.AddTransient<GetSavedApplicationsQueryHandler>();

        services.AddTransient<GetDemoAccountQueryHandler>();

        services.AddTransient<GrantSubscriptionCommandHandler>();

        return services;
    }

    public static IServiceCollection AddAuthenticationServices(
        this IServiceCollection services, IConfiguration configuration)
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
                options.MapInboundClaims = false;
            });

        services.AddAuthorizationBuilder()
            .AddFallbackPolicy("Authenticated", policy => policy.RequireAuthenticatedUser());

        return services;
    }
}
