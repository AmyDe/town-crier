using TownCrier.Application.RateLimiting;
using TownCrier.Web;
using TownCrier.Web.DependencyInjection;

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

builder.Services
    .AddInfrastructureServices(builder.Configuration)
    .AddApplicationServices()
    .AddAuthenticationServices(builder.Configuration);

builder.Services.Configure<RateLimitOptions>(builder.Configuration.GetSection("RateLimiting"));

var app = builder.Build();

app.UseMiddlewarePipeline();
app.MapAllEndpoints();

await app.RunAsync().ConfigureAwait(false);
