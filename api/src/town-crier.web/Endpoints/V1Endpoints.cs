namespace TownCrier.Web.Endpoints;

internal static class V1Endpoints
{
    public static WebApplication MapV1Endpoints(this WebApplication app)
    {
        var v1 = app.MapGroup("/v1");

        v1.MapHealthEndpoints();
        v1.MapVersionConfigEndpoints();
        v1.MapDesignationEndpoints();
        v1.MapAuthorityEndpoints();
        v1.MapApplicationEndpoints();
        v1.MapGeocodeEndpoints();
        v1.MapUserProfileEndpoints();
        v1.MapDeviceTokenEndpoints();
        v1.MapSearchEndpoints();
        v1.MapNotificationEndpoints();
        v1.MapSavedApplicationEndpoints();
        v1.MapWatchZoneEndpoints();
        v1.MapGroupEndpoints();
        v1.MapDemoAccountEndpoints();

        return app;
    }
}
