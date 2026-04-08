using TownCrier.Application.Health;
using TownCrier.Web.Endpoints;
using TownCrier.Web.Observability;
using TownCrier.Web.RateLimiting;

namespace TownCrier.Web.Extensions;

internal static class WebApplicationExtensions
{
    public static void UseMiddlewarePipeline(this WebApplication app)
    {
        app.UseCors();
        app.UseMiddleware<ErrorResponseMiddleware>();
        app.UseAuthentication();
        app.UseAuthorization();
        app.UseMiddleware<RateLimitMiddleware>();
    }

    public static void MapAllEndpoints(this WebApplication app)
    {
        app.MapHealthEndpoints();

        var v1 = app.MapGroup("/v1");
        v1.MapGet("/health", () =>
            CheckHealthQueryHandler.HandleAsync(new CheckHealthQuery(), CancellationToken.None))
            .AllowAnonymous();

        v1.MapVersionConfigEndpoints();
        v1.MapLegalEndpoints();
        v1.MapDesignationEndpoints();
        v1.MapAuthorityEndpoints();
        v1.MapPlanningApplicationEndpoints();
        v1.MapGeocodeEndpoints();
        v1.MapUserProfileEndpoints();
        v1.MapDeviceTokenEndpoints();
        v1.MapSearchEndpoints();
        v1.MapNotificationEndpoints();
        v1.MapSavedApplicationEndpoints();
        v1.MapWatchZoneEndpoints();
        v1.MapDemoAccountEndpoints();
        v1.MapAdminEndpoints();

        app.MapApiEndpoints();
    }
}
