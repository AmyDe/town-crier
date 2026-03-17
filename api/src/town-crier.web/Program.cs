using System.Security.Claims;
using Microsoft.AspNetCore.Authentication.JwtBearer;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Geocoding;
using TownCrier.Application.Health;
using TownCrier.Application.PlanIt;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.Polling;
using TownCrier.Application.RateLimiting;
using TownCrier.Application.Search;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Infrastructure.DeviceRegistrations;
using TownCrier.Infrastructure.Geocoding;
using TownCrier.Infrastructure.PlanIt;
using TownCrier.Infrastructure.PlanningApplications;
using TownCrier.Infrastructure.Polling;
using TownCrier.Infrastructure.RateLimiting;
using TownCrier.Infrastructure.UserProfiles;
using TownCrier.Infrastructure.WatchZones;
using TownCrier.Web;
using TownCrier.Web.Observability;
using TownCrier.Web.Polling;
using TownCrier.Web.RateLimiting;

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
builder.Services.AddSingleton<IPlanningApplicationRepository, InMemoryPlanningApplicationRepository>();
builder.Services.AddSingleton<IActiveAuthorityProvider, InMemoryActiveAuthorityProvider>();
builder.Services.AddSingleton<IPollingHealthStore, InMemoryPollingHealthStore>();
builder.Services.AddSingleton<IPollingHealthAlerter, LogPollingHealthAlerter>();
builder.Services.AddSingleton(new PollingHealthConfig(
    StalenessThreshold: TimeSpan.FromHours(1),
    MaxConsecutiveFailures: 5));
builder.Services.AddSingleton(TimeProvider.System);
builder.Services.AddSingleton<IWatchZoneRepository, InMemoryWatchZoneRepository>();
builder.Services.AddSingleton<INotificationEnqueuer, LogNotificationEnqueuer>();
builder.Services.AddTransient<PollPlanItCommandHandler>();
builder.Services.AddHostedService<PlanItPollingService>();

builder.Services.AddSingleton<IUserProfileRepository, InMemoryUserProfileRepository>();
builder.Services.AddTransient<CreateUserProfileCommandHandler>();
builder.Services.AddTransient<GetUserProfileQueryHandler>();
builder.Services.AddTransient<UpdateUserProfileCommandHandler>();
builder.Services.AddTransient<ExportUserDataQueryHandler>();
builder.Services.AddTransient<DeleteUserProfileCommandHandler>();

builder.Services.AddSingleton<IDeviceRegistrationRepository, InMemoryDeviceRegistrationRepository>();
builder.Services.AddTransient<RegisterDeviceTokenCommandHandler>();
builder.Services.AddTransient<RemoveInvalidDeviceTokenCommandHandler>();

builder.Services.AddTransient<SearchPlanningApplicationsQueryHandler>();

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

builder.Services.AddSingleton<IRateLimitStore, InMemoryRateLimitStore>();
builder.Services.Configure<RateLimitOptions>(builder.Configuration.GetSection("RateLimiting"));

var app = builder.Build();

app.UseMiddleware<CorrelationIdMiddleware>();
app.UseMiddleware<ErrorResponseMiddleware>();
app.UseMiddleware<RequestLoggingMiddleware>();
app.UseAuthentication();
app.UseAuthorization();
app.UseMiddleware<RateLimitMiddleware>();

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

v1.MapPost("/me", async (ClaimsPrincipal user, CreateUserProfileCommandHandler handler, CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    var result = await handler.HandleAsync(new CreateUserProfileCommand(userId), ct).ConfigureAwait(false);
    return Results.Ok(result);
});

v1.MapGet("/me", async (ClaimsPrincipal user, GetUserProfileQueryHandler handler, CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    var result = await handler.HandleAsync(new GetUserProfileQuery(userId), ct).ConfigureAwait(false);
    return result is null ? Results.NotFound() : Results.Ok(result);
});

v1.MapPatch("/me", async (ClaimsPrincipal user, UpdateUserProfileCommand command, UpdateUserProfileCommandHandler handler, CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    var profileCommand = new UpdateUserProfileCommand(userId, command.Postcode, command.PushEnabled);

    try
    {
        var result = await handler.HandleAsync(profileCommand, ct).ConfigureAwait(false);
        return Results.Ok(result);
    }
    catch (UserProfileNotFoundException)
    {
        return Results.NotFound();
    }
});

v1.MapGet("/me/data", async (ClaimsPrincipal user, ExportUserDataQueryHandler handler, CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    var result = await handler.HandleAsync(new ExportUserDataQuery(userId), ct).ConfigureAwait(false);
    return result is null ? Results.NotFound() : Results.Ok(result);
});

v1.MapDelete("/me", async (ClaimsPrincipal user, DeleteUserProfileCommandHandler handler, CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;

    try
    {
        await handler.HandleAsync(new DeleteUserProfileCommand(userId), ct).ConfigureAwait(false);
        return Results.NoContent();
    }
    catch (UserProfileNotFoundException)
    {
        return Results.NotFound();
    }
});

v1.MapPut("/me/device-token", async (ClaimsPrincipal user, RegisterDeviceTokenRequest request, RegisterDeviceTokenCommandHandler handler, CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    var command = new RegisterDeviceTokenCommand(userId, request.Token, request.Platform);
    await handler.HandleAsync(command, ct).ConfigureAwait(false);
    return Results.NoContent();
});

v1.MapDelete("/me/device-token/{token}", async (ClaimsPrincipal user, string token, RemoveInvalidDeviceTokenCommandHandler handler, CancellationToken ct) =>
{
    var command = new RemoveInvalidDeviceTokenCommand(token);
    await handler.HandleAsync(command, ct).ConfigureAwait(false);
    return Results.NoContent();
});

v1.MapGet("/search", async (ClaimsPrincipal user, string q, int authorityId, int page, SearchPlanningApplicationsQueryHandler handler, CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    var query = new SearchPlanningApplicationsQuery(userId, q, authorityId, page);

    try
    {
        var result = await handler.HandleAsync(query, ct).ConfigureAwait(false);
        return Results.Ok(result);
    }
    catch (ProTierRequiredException)
    {
        return Results.Json(new { error = "This feature requires a Pro subscription." }, statusCode: 403);
    }
    catch (UserProfileNotFoundException)
    {
        return Results.NotFound();
    }
});

var api = app.MapGroup("/api");

api.MapGet("/me", (ClaimsPrincipal user) =>
{
    var userId = user.FindFirstValue("sub")!;
    return Results.Ok(new { userId });
});

await app.RunAsync().ConfigureAwait(false);
