using Microsoft.AspNetCore.Authentication.JwtBearer;
using TownCrier.Application.Admin;
using TownCrier.Application.Auth;
using TownCrier.Application.Authorities;
using TownCrier.Application.DemoAccount;
using TownCrier.Application.Designations;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Geocoding;
using TownCrier.Application.Notifications;
using TownCrier.Application.OfferCodes;
using TownCrier.Application.PlanIt;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.RateLimiting;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.Search;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Infrastructure.Auth;
using TownCrier.Infrastructure.Authorities;
using TownCrier.Infrastructure.Cosmos;
using TownCrier.Infrastructure.DeviceRegistrations;
using TownCrier.Infrastructure.Geocoding;
using TownCrier.Infrastructure.GovUkPlanningData;
using TownCrier.Infrastructure.Notifications;
using TownCrier.Infrastructure.OfferCodes;
using TownCrier.Infrastructure.PlanIt;
using TownCrier.Infrastructure.PlanningApplications;
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

        services.AddSingleton<IPlanningApplicationRepository, CosmosPlanningApplicationRepository>();
        services.AddSingleton<IUserProfileRepository, CosmosUserProfileRepository>();
        services.AddSingleton<IWatchZoneRepository, CosmosWatchZoneRepository>();
        services.AddSingleton<ISavedApplicationRepository, CosmosSavedApplicationRepository>();
        services.AddSingleton<IDeviceRegistrationRepository, CosmosDeviceRegistrationRepository>();
        services.AddSingleton<INotificationRepository, CosmosNotificationRepository>();
        services.AddSingleton<IOfferCodeRepository, CosmosOfferCodeRepository>();
        services.AddSingleton<IOfferCodeGenerator, OfferCodeGenerator>();

        var acsConnectionString = configuration["AzureCommunicationServices:ConnectionString"];
        if (!string.IsNullOrEmpty(acsConnectionString))
        {
            try
            {
                services.AddSingleton<IEmailSender>(sp =>
                    new AcsEmailSender(acsConnectionString, sp.GetRequiredService<ILogger<AcsEmailSender>>()));
            }
            catch (InvalidOperationException)
            {
                services.AddSingleton<IEmailSender, NoOpEmailSender>();
            }
        }
        else
        {
            services.AddSingleton<IEmailSender, NoOpEmailSender>();
        }

        var auth0M2mClientId = configuration["Auth0:M2M:ClientId"];
        var auth0M2mClientSecret = configuration["Auth0:M2M:ClientSecret"];
        var auth0Domain = configuration["Auth0:Domain"];
        if (!string.IsNullOrEmpty(auth0M2mClientId) && !string.IsNullOrEmpty(auth0M2mClientSecret) && !string.IsNullOrEmpty(auth0Domain))
        {
            services.AddHttpClient<IAuth0ManagementClient, Auth0ManagementClient>((httpClient, sp) =>
                new Auth0ManagementClient(httpClient, auth0Domain, auth0M2mClientId, auth0M2mClientSecret));
        }
        else
        {
            services.AddSingleton<IAuth0ManagementClient, NoOpAuth0ManagementClient>();
        }

        services.AddSingleton(TimeProvider.System);

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

        var planItThrottle = new PlanItThrottleOptions();
        configuration.GetSection("PlanIt:Throttle").Bind(planItThrottle);
        services.AddSingleton(planItThrottle);

        var planItRetry = new PlanItRetryOptions();
        configuration.GetSection("PlanIt:Retry").Bind(planItRetry);
        services.AddSingleton(planItRetry);

#pragma warning disable S1075 // Hardcoded URI is a sensible default
        var planItBaseUrl = configuration["PlanIt:BaseUrl"] ?? "https://www.planit.org.uk/";
#pragma warning restore S1075
        services.AddHttpClient<IPlanItClient, PlanItClient>(client =>
        {
            client.BaseAddress = new Uri(planItBaseUrl);
        });
        services.AddSingleton<IAuthorityProvider>(new StaticAuthorityProvider());

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
        services.AddTransient<RecordUserActivityCommandHandler>();

        services.AddTransient<CreateWatchZoneCommandHandler>();
        services.AddTransient<UpdateWatchZoneCommandHandler>();
        services.AddTransient<ListWatchZonesQueryHandler>();
        services.AddTransient<DeleteWatchZoneCommandHandler>();
        services.AddTransient<GetApplicationsByZoneQueryHandler>();

        services.AddTransient<RegisterDeviceTokenCommandHandler>();
        services.AddTransient<RemoveInvalidDeviceTokenCommandHandler>();

        services.AddTransient<GetApplicationByUidQueryHandler>();
        services.AddTransient<GetUserApplicationAuthoritiesQueryHandler>();
        services.AddTransient<SearchPlanningApplicationsQueryHandler>();

        services.AddTransient<GetNotificationsQueryHandler>();

        services.AddTransient<SaveApplicationCommandHandler>();
        services.AddTransient<RemoveSavedApplicationCommandHandler>();
        services.AddTransient<GetSavedApplicationsQueryHandler>();

        services.AddTransient<GetDemoAccountQueryHandler>();

        services.AddTransient<GrantSubscriptionCommandHandler>();
        services.AddTransient<ListUsersQueryHandler>();

        services.AddTransient<GenerateOfferCodesCommandHandler>();
        services.AddTransient<RedeemOfferCodeCommandHandler>();

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
