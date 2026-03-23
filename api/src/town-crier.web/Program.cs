using System.Security.Claims;
using Microsoft.AspNetCore.Authentication.JwtBearer;
using TownCrier.Application.Authorities;
using TownCrier.Application.DemoAccount;
using TownCrier.Application.Designations;
using TownCrier.Application.DeviceRegistrations;
using TownCrier.Application.Geocoding;
using TownCrier.Application.Groups;
using TownCrier.Application.Health;
using TownCrier.Application.Notifications;
using TownCrier.Application.PlanIt;
using TownCrier.Application.PlanningApplications;
using TownCrier.Application.Polling;
using TownCrier.Application.RateLimiting;
using TownCrier.Application.SavedApplications;
using TownCrier.Application.Search;
using TownCrier.Application.UserProfiles;
using TownCrier.Application.WatchZones;
using TownCrier.Domain.Groups;
using TownCrier.Domain.Polling;
using TownCrier.Domain.UserProfiles;
using TownCrier.Infrastructure.Cosmos;
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
using TownCrier.Web;
using TownCrier.Web.Observability;
using TownCrier.Web.Polling;
using TownCrier.Web.RateLimiting;

var builder = WebApplication.CreateSlimBuilder(args);

builder.Logging.AddJsonConsole();
builder.Services.AddCosmosClient(builder.Configuration);

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
builder.Services.AddSingleton<IAuthorityProvider>(sp =>
{
    var factory = sp.GetRequiredService<IHttpClientFactory>();
    var httpClient = factory.CreateClient("PlanItAreas");
    httpClient.BaseAddress = new Uri(planItBaseUrl);
    return new CachedPlanItAuthorityProvider(httpClient, sp.GetRequiredService<TimeProvider>());
});
builder.Services.AddTransient<GetAuthoritiesQueryHandler>();
builder.Services.AddTransient<GetAuthorityByIdQueryHandler>();

#pragma warning disable S1075 // Hardcoded URI is a sensible default
var govUkBaseUrl = builder.Configuration["GovUkPlanningData:BaseUrl"] ?? "https://www.planning.data.gov.uk/";
#pragma warning restore S1075
builder.Services.AddHttpClient<IDesignationDataProvider, GovUkPlanningDataClient>(client =>
{
    client.BaseAddress = new Uri(govUkBaseUrl);
});
builder.Services.AddTransient<GetDesignationContextQueryHandler>();

var pollStateFilePath = builder.Configuration["Polling:StateFilePath"] ?? Path.Combine(AppContext.BaseDirectory, "poll-state.txt");
builder.Services.AddSingleton<IPollStateStore>(new FilePollStateStore(pollStateFilePath));
builder.Services.AddCosmosClient(builder.Configuration);
builder.Services.AddSingleton<IPlanningApplicationRepository, CosmosPlanningApplicationRepository>();
builder.Services.AddSingleton<IActiveAuthorityProvider, InMemoryActiveAuthorityProvider>();
builder.Services.AddSingleton<IPollingHealthStore, InMemoryPollingHealthStore>();
builder.Services.AddSingleton<IPollingHealthAlerter, LogPollingHealthAlerter>();
builder.Services.AddSingleton(new PollingHealthConfig(
    StalenessThreshold: TimeSpan.FromHours(1),
    MaxConsecutiveFailures: 5));
builder.Services.AddSingleton(TimeProvider.System);
builder.Services.AddSingleton<IWatchZoneRepository, CosmosWatchZoneRepository>();
builder.Services.AddSingleton<INotificationEnqueuer, LogNotificationEnqueuer>();
builder.Services.AddSingleton(new PollingScheduleConfig(
    HighThreshold: builder.Configuration.GetValue("Polling:HighThreshold", 5),
    LowThreshold: builder.Configuration.GetValue("Polling:LowThreshold", 2)));
builder.Services.AddTransient<PollPlanItCommandHandler>();
builder.Services.AddHostedService<PlanItPollingService>();

builder.Services.AddSingleton<IUserProfileRepository, InMemoryUserProfileRepository>();
builder.Services.AddTransient<CreateUserProfileCommandHandler>();
builder.Services.AddTransient<GetUserProfileQueryHandler>();
builder.Services.AddTransient<UpdateUserProfileCommandHandler>();
builder.Services.AddTransient<ExportUserDataQueryHandler>();
builder.Services.AddTransient<DeleteUserProfileCommandHandler>();
builder.Services.AddTransient<UpdateZonePreferencesCommandHandler>();
builder.Services.AddTransient<GetZonePreferencesQueryHandler>();

builder.Services.AddSingleton<IDeviceRegistrationRepository>(sp =>
    new CosmosDeviceRegistrationRepository(sp.GetRequiredService<Microsoft.Azure.Cosmos.CosmosClient>()));
builder.Services.AddTransient<RegisterDeviceTokenCommandHandler>();
builder.Services.AddTransient<RemoveInvalidDeviceTokenCommandHandler>();

builder.Services.AddTransient<SearchPlanningApplicationsQueryHandler>();

builder.Services.AddSingleton<INotificationRepository>(sp =>
    new CosmosNotificationRepository(sp.GetRequiredService<Microsoft.Azure.Cosmos.CosmosClient>()));
builder.Services.AddTransient<GetNotificationsQueryHandler>();

builder.Services.AddSingleton<ISavedApplicationRepository, CosmosSavedApplicationRepository>();
builder.Services.AddTransient<SaveApplicationCommandHandler>();
builder.Services.AddTransient<RemoveSavedApplicationCommandHandler>();
builder.Services.AddTransient<GetSavedApplicationsQueryHandler>();

builder.Services.AddTransient<GetDemoAccountQueryHandler>();

builder.Services.AddSingleton<IGroupRepository, InMemoryGroupRepository>();
builder.Services.AddSingleton<IGroupInvitationRepository, InMemoryGroupInvitationRepository>();
builder.Services.AddTransient<CreateGroupCommandHandler>();
builder.Services.AddTransient<GetGroupQueryHandler>();
builder.Services.AddTransient<GetUserGroupsQueryHandler>();
builder.Services.AddTransient<InviteMemberCommandHandler>();
builder.Services.AddTransient<AcceptInvitationCommandHandler>();
builder.Services.AddTransient<RemoveGroupMemberCommandHandler>();
builder.Services.AddTransient<DeleteGroupCommandHandler>();

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

v1.MapGet("/designations", async (
    double latitude,
    double longitude,
    GetDesignationContextQueryHandler handler,
    CancellationToken ct) =>
{
    var query = new GetDesignationContextQuery(latitude, longitude);
    var result = await handler.HandleAsync(query, ct).ConfigureAwait(false);
    return Results.Ok(result);
});

v1.MapGet("/authorities", async (
    string? search,
    GetAuthoritiesQueryHandler handler,
    CancellationToken ct) =>
{
    var result = await handler.HandleAsync(new GetAuthoritiesQuery(search), ct).ConfigureAwait(false);
    return Results.Ok(result);
}).AllowAnonymous();

v1.MapGet("/authorities/{id:int}", async (
    int id,
    GetAuthorityByIdQueryHandler handler,
    CancellationToken ct) =>
{
    var result = await handler.HandleAsync(new GetAuthorityByIdQuery(id), ct).ConfigureAwait(false);
    return result is null ? Results.NotFound() : Results.Ok(result);
}).AllowAnonymous();

v1.MapGet("/geocode/{postcode}", async (string postcode, GeocodePostcodeQueryHandler handler, CancellationToken ct) =>
{
    try
    {
        var result = await handler.HandleAsync(new GeocodePostcodeQuery(postcode), ct).ConfigureAwait(false);
        return Results.Ok(result);
    }
    catch (ArgumentException ex)
    {
        return Results.BadRequest(new ApiErrorResponse(ex.Message));
    }
    catch (InvalidOperationException ex)
    {
        return Results.NotFound(new ApiErrorResponse(ex.Message));
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
        return Results.Json(new ApiErrorResponse("This feature requires a Pro subscription."), AppJsonSerializerContext.Default.ApiErrorResponse, statusCode: 403);
    }
    catch (UserProfileNotFoundException)
    {
        return Results.NotFound();
    }
});

v1.MapGet("/notifications", async (
    ClaimsPrincipal user,
    int? page,
    int? pageSize,
    GetNotificationsQueryHandler handler,
    CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    var query = new GetNotificationsQuery(userId, page ?? 1, pageSize ?? 20);
    var result = await handler.HandleAsync(query, ct).ConfigureAwait(false);
    return Results.Ok(result);
});

v1.MapPut("/me/saved-applications/{applicationUid}", async (
    ClaimsPrincipal user,
    string applicationUid,
    SaveApplicationCommandHandler handler,
    CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    await handler.HandleAsync(new SaveApplicationCommand(userId, applicationUid), ct).ConfigureAwait(false);
    return Results.NoContent();
});

v1.MapDelete("/me/saved-applications/{applicationUid}", async (
    ClaimsPrincipal user,
    string applicationUid,
    RemoveSavedApplicationCommandHandler handler,
    CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    await handler.HandleAsync(new RemoveSavedApplicationCommand(userId, applicationUid), ct).ConfigureAwait(false);
    return Results.NoContent();
});

v1.MapGet("/me/saved-applications", async (
    ClaimsPrincipal user,
    GetSavedApplicationsQueryHandler handler,
    CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    var result = await handler.HandleAsync(new GetSavedApplicationsQuery(userId), ct).ConfigureAwait(false);
    return Results.Ok(result);
});

v1.MapGet("/me/watch-zones/{zoneId}/preferences", async (
    ClaimsPrincipal user,
    string zoneId,
    GetZonePreferencesQueryHandler handler,
    CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;

    try
    {
        var result = await handler.HandleAsync(
            new GetZonePreferencesQuery(userId, zoneId), ct).ConfigureAwait(false);
        return Results.Ok(result);
    }
    catch (UserProfileNotFoundException)
    {
        return Results.NotFound();
    }
});

v1.MapPut("/me/watch-zones/{zoneId}/preferences", async (
    ClaimsPrincipal user,
    string zoneId,
    UpdateZonePreferencesCommand command,
    UpdateZonePreferencesCommandHandler handler,
    CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    var fullCommand = new UpdateZonePreferencesCommand(
        userId,
        zoneId,
        command.NewApplications,
        command.StatusChanges,
        command.DecisionUpdates);

    try
    {
        var result = await handler.HandleAsync(fullCommand, ct).ConfigureAwait(false);
        return Results.Ok(result);
    }
    catch (UserProfileNotFoundException)
    {
        return Results.NotFound();
    }
    catch (InsufficientTierException)
    {
        return Results.Json(new ApiErrorResponse("This feature requires a Pro subscription."), AppJsonSerializerContext.Default.ApiErrorResponse, statusCode: 403);
    }
});

// Groups
v1.MapPost("/groups", async (
    ClaimsPrincipal user,
    CreateGroupCommand command,
    CreateGroupCommandHandler handler,
    CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    var fullCommand = new CreateGroupCommand(
        userId,
        command.GroupId,
        command.Name,
        command.Latitude,
        command.Longitude,
        command.RadiusMetres,
        command.AuthorityId);
    var result = await handler.HandleAsync(fullCommand, ct).ConfigureAwait(false);
    return Results.Created($"/v1/groups/{result.GroupId}", result);
});

v1.MapGet("/groups", async (
    ClaimsPrincipal user,
    GetUserGroupsQueryHandler handler,
    CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    var result = await handler.HandleAsync(new GetUserGroupsQuery(userId), ct).ConfigureAwait(false);
    return Results.Ok(result);
});

v1.MapGet("/groups/{groupId}", async (
    ClaimsPrincipal user,
    string groupId,
    GetGroupQueryHandler handler,
    CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;

    try
    {
        var result = await handler.HandleAsync(
            new GetGroupQuery(userId, groupId), ct).ConfigureAwait(false);
        return Results.Ok(result);
    }
    catch (GroupNotFoundException)
    {
        return Results.NotFound();
    }
});

v1.MapDelete("/groups/{groupId}", async (
    ClaimsPrincipal user,
    string groupId,
    DeleteGroupCommandHandler handler,
    CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;

    try
    {
        await handler.HandleAsync(
            new DeleteGroupCommand(userId, groupId), ct).ConfigureAwait(false);
        return Results.NoContent();
    }
    catch (GroupNotFoundException)
    {
        return Results.NotFound();
    }
    catch (UnauthorizedGroupOperationException)
    {
        return Results.Json(new ApiErrorResponse("Only the group owner can delete the group."), AppJsonSerializerContext.Default.ApiErrorResponse, statusCode: 403);
    }
});

v1.MapPost("/groups/{groupId}/invitations", async (
    ClaimsPrincipal user,
    string groupId,
    InviteMemberCommand command,
    InviteMemberCommandHandler handler,
    CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;
    var fullCommand = new InviteMemberCommand(
        userId,
        groupId,
        command.InvitationId,
        command.InviteeEmail);

    try
    {
        var result = await handler.HandleAsync(fullCommand, ct).ConfigureAwait(false);
        return Results.Created($"/v1/groups/{groupId}/invitations/{result.InvitationId}", result);
    }
    catch (GroupNotFoundException)
    {
        return Results.NotFound();
    }
    catch (UnauthorizedGroupOperationException)
    {
        return Results.Json(new ApiErrorResponse("Only the group owner can invite members."), AppJsonSerializerContext.Default.ApiErrorResponse, statusCode: 403);
    }
});

v1.MapPost("/invitations/{invitationId}/accept", async (
    ClaimsPrincipal user,
    string invitationId,
    AcceptInvitationCommandHandler handler,
    CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;

    try
    {
        await handler.HandleAsync(
            new AcceptInvitationCommand(userId, invitationId), ct).ConfigureAwait(false);
        return Results.NoContent();
    }
    catch (InvalidOperationException ex)
    {
        return Results.BadRequest(new ApiErrorResponse(ex.Message));
    }
    catch (GroupNotFoundException)
    {
        return Results.NotFound();
    }
});

v1.MapDelete("/groups/{groupId}/members/{memberUserId}", async (
    ClaimsPrincipal user,
    string groupId,
    string memberUserId,
    RemoveGroupMemberCommandHandler handler,
    CancellationToken ct) =>
{
    var userId = user.FindFirstValue("sub")!;

    try
    {
        await handler.HandleAsync(
            new RemoveGroupMemberCommand(userId, groupId, memberUserId), ct).ConfigureAwait(false);
        return Results.NoContent();
    }
    catch (GroupNotFoundException)
    {
        return Results.NotFound();
    }
    catch (UnauthorizedGroupOperationException)
    {
        return Results.Json(new ApiErrorResponse("Only the group owner can remove members."), AppJsonSerializerContext.Default.ApiErrorResponse, statusCode: 403);
    }
});

v1.MapGet("/demo-account", async (
    GetDemoAccountQueryHandler handler,
    CancellationToken ct) =>
{
    var result = await handler.HandleAsync(new GetDemoAccountQuery(), ct).ConfigureAwait(false);
    return Results.Ok(result);
}).AllowAnonymous();

var api = app.MapGroup("/api");

api.MapGet("/me", (ClaimsPrincipal user) =>
{
    var userId = user.FindFirstValue("sub")!;
    return Results.Ok(new UserIdResponse(userId));
});

await app.RunAsync().ConfigureAwait(false);
