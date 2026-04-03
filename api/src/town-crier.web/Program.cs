using TownCrier.Web;
using TownCrier.Web.Extensions;
using TownCrier.Web.Polling;

var builder = WebApplication.CreateSlimBuilder(args);

builder.Logging.AddJsonConsole();

var allowedOrigins = builder.Configuration.GetSection("Cors:AllowedOrigins").Get<string[]>()
    ?? ["http://localhost:5173"];
builder.Services.AddCors(options =>
{
    options.AddDefaultPolicy(policy =>
    {
        policy.WithOrigins(allowedOrigins)
            .AllowAnyHeader()
            .AllowAnyMethod();
    });
});

builder.Services.ConfigureHttpJsonOptions(options =>
{
    options.SerializerOptions.TypeInfoResolverChain.Insert(0, AppJsonSerializerContext.Default);
});

builder.Services.AddInfrastructureServices(builder.Configuration);
builder.Services.AddApplicationServices(builder.Configuration);
builder.Services.AddAuthenticationServices(builder.Configuration);
builder.Services.AddHostedService<PlanItPollingService>();

var app = builder.Build();

app.UseMiddlewarePipeline();
app.MapAllEndpoints();

await app.RunAsync().ConfigureAwait(false);
