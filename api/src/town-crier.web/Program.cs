using Microsoft.AspNetCore.Authentication.JwtBearer;
using TownCrier.Application.Geocoding;
using TownCrier.Application.Health;
using TownCrier.Infrastructure.Geocoding;
using TownCrier.Web;
using TownCrier.Web.Observability;

var builder = WebApplication.CreateSlimBuilder(args);

builder.Services.ConfigureHttpJsonOptions(options =>
{
    options.SerializerOptions.TypeInfoResolverChain.Insert(0, AppJsonSerializerContext.Default);
});

#pragma warning disable S1075 // Hardcoded URI is a sensible default
var postcodesIoBaseUrl = builder.Configuration["PostcodesIo:BaseUrl"] ?? "https://api.postcodes.io/";
#pragma warning restore S1075
builder.Services.AddHttpClient<IPostcodeGeocoder, PostcodesIoGeocoder>(client =>
{
    client.BaseAddress = new Uri(postcodesIoBaseUrl);
});
builder.Services.AddTransient<GeocodePostcodeQueryHandler>();

builder.Services.AddAuthentication(JwtBearerDefaults.AuthenticationScheme)
    .AddJwtBearer(options =>
    {
        var domain = builder.Configuration["Auth0:Domain"]
            ?? throw new InvalidOperationException("Auth0:Domain configuration is required.");
        var audience = builder.Configuration["Auth0:Audience"]
            ?? throw new InvalidOperationException("Auth0:Audience configuration is required.");

        options.Authority = $"https://{domain}/";
        options.Audience = audience;
    });

builder.Services.AddAuthorizationBuilder()
    .AddFallbackPolicy("Authenticated", policy => policy.RequireAuthenticatedUser());

var app = builder.Build();

app.UseMiddleware<CorrelationIdMiddleware>();
app.UseAuthentication();
app.UseAuthorization();

app.MapGet("/health", () => CheckHealthQueryHandler.HandleAsync(new CheckHealthQuery(), CancellationToken.None))
    .AllowAnonymous();

var v1 = app.MapGroup("/v1");
v1.MapGet("/health", () => CheckHealthQueryHandler.HandleAsync(new CheckHealthQuery(), CancellationToken.None))
    .AllowAnonymous();

v1.MapGet("/geocode/{postcode}", async (string postcode, GeocodePostcodeQueryHandler handler, CancellationToken ct) =>
{
    try
    {
        var result = await handler.HandleAsync(new GeocodePostcodeQuery(postcode), ct).ConfigureAwait(false);
        return Results.Ok(result);
    }
    catch (ArgumentException ex)
    {
        return Results.BadRequest(new { error = ex.Message });
    }
    catch (InvalidOperationException ex)
    {
        return Results.NotFound(new { error = ex.Message });
    }
});

app.MapGet("/api/me", () => Results.Ok())
    .RequireAuthorization();

await app.RunAsync().ConfigureAwait(false);
