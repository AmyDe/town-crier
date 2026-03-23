using Microsoft.AspNetCore.Authentication.JwtBearer;
using Microsoft.AspNetCore.Hosting;
using Microsoft.AspNetCore.Mvc.Testing;
using Microsoft.AspNetCore.TestHost;
using Microsoft.Extensions.DependencyInjection;
using Microsoft.IdentityModel.Protocols.OpenIdConnect;
using Microsoft.IdentityModel.Tokens;
using TownCrier.Application.Groups;
using TownCrier.Infrastructure.Groups;
using TownCrier.Web.Tests.Auth;

namespace TownCrier.Web.Tests;

internal sealed class TestWebApplicationFactory : WebApplicationFactory<Program>
{
    protected override void ConfigureWebHost(IWebHostBuilder builder)
    {
        builder.UseSetting("Auth0:Domain", "test.auth0.com");
        builder.UseSetting("Auth0:Audience", "https://api.towncrier.app");
        builder.UseSetting("ConnectionStrings:CosmosDb", "AccountEndpoint=https://localhost:8081/;AccountKey=dGVzdA==");

        builder.ConfigureTestServices(services =>
        {
            services.AddSingleton<IGroupRepository, InMemoryGroupRepository>();
            services.AddSingleton<IGroupInvitationRepository, InMemoryGroupInvitationRepository>();

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
