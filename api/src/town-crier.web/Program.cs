using Microsoft.AspNetCore.Authentication.JwtBearer;
using TownCrier.Application.Geocoding;
using TownCrier.Application.Health;
using TownCrier.Application.PlanIt;
using TownCrier.Application.Polling;
using TownCrier.Infrastructure.Geocoding;
using TownCrier.Infrastructure.PlanIt;
using TownCrier.Infrastructure.Polling;
using TownCrier.Web;
using TownCrier.Web.Observability;
using TownCrier.Web.Polling;

var builder = WebApplication.CreateSlimBuilder(args);

builder.Logging.AddJsonConsole();

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

#pragma warning disable S1075 // Hardcoded URI is a sensible default
var planItBaseUrl = builder.Configuration["PlanIt:BaseUrl"] ?? "https://www.planit.org.uk/";
#pragma warning restore S1075
builder.Services.AddHttpClient<IPlanItClient, PlanItClient>(client =>
{
    client.BaseAddress = new Uri(planItBaseUrl);
});
var pollStateFilePath = builder.Configuration["Polling:StateFilePath"] ?? Path.Combine(AppContext.BaseDirectory, "poll-state.txt");
builder.Services.AddSingleton<IPollStateStore>(new FilePollStateStore(pollStateFilePath));
builder.Services.AddSingleton(TimeProvider.System);
builder.Services.AddTransient<PollPlanItCommandHandler>();
builder.Services.AddHostedService<PlanItPollingService>();

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
app.UseMiddleware<RequestLoggingMiddleware>();
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
