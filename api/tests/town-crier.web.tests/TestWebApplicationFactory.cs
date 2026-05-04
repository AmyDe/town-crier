using Microsoft.AspNetCore.Authentication.JwtBearer;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.IdentityModel.Protocols.OpenIdConnect;
using Microsoft.IdentityModel.Tokens;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Notifications;
using TownCrier.Application.NotificationState;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Infrastructure.DeviceRegistrations;
using TownCrier.Infrastructure.Notifications;
using TownCrier.Infrastructure.NotificationState;
using TownCrier.Infrastructure.PlanningApplications;
using TownCrier.Infrastructure.SavedApplications;
using TownCrier.Infrastructure.UserProfiles;
using TownCrier.Infrastructure.WatchZones;
using TownCrier.Web.Tests.Auth;

namespace TownCrier.Web.Tests;

internal sealed class TestWebApplicationFactory : WebApplicationFactory<Program>
{
    protected override void ConfigureWebHost(IWebHostBuilder builder)
    {
        builder.UseSetting("Auth0:Domain", "test.auth0.com");
        builder.UseSetting("Auth0:Audience", "https://api.towncrier.app");
        builder.UseSetting("Cors:AllowedOrigins:0", "http://localhost:5173");
        builder.UseSetting("Cosmos:AccountEndpoint", "https://test-account.documents.azure.com:443");
        builder.UseSetting("Cosmos:DatabaseName", "town-crier");

        builder.ConfigureTestServices(services =>
        {
            services.AddSingleton<IUserProfileRepository, InMemoryUserProfileRepository>();
            services.AddSingleton<IPlanningApplicationRepository, InMemoryPlanningApplicationRepository>();
            services.AddSingleton<IWatchZoneRepository, InMemoryWatchZoneRepository>();
            services.AddSingleton<IDeviceRegistrationRepository, InMemoryDeviceRegistrationRepository>();
            services.AddSingleton<INotificationRepository, InMemoryNotificationRepository>();
            services.AddSingleton<INotificationStateRepository, InMemoryNotificationStateRepository>();
            services.AddSingleton<ISavedApplicationRepository, InMemorySavedApplicationRepository>();

            services.PostConfigure<JwtBearerOptions>(JwtBearerDefaults.AuthenticationScheme, options =>
            {
                options.Authority = null;
                options.ConfigurationManager = null;
                options.Configuration = new OpenIdConnectConfiguration
                {
                    Issuer = "https://test.auth0.com/",
                };
                options.Configuration.SigningKeys.Add(TestJwtToken.SecurityKey);
                options.TokenValidationParameters = new TokenValidationParameters
                {
                    ValidateIssuer = true,
                    ValidIssuer = "https://test.auth0.com/",
                    ValidateAudience = true,
                    ValidAudience = "https://api.towncrier.app",
                    ValidateLifetime = true,
                    ValidateIssuerSigningKey = true,
                    IssuerSigningKey = TestJwtToken.SecurityKey,
                };
            });
        });
    }
}
