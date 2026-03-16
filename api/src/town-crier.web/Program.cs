using TownCrier.Application.Health;
using TownCrier.Web;

var builder = WebApplication.CreateSlimBuilder(args);

builder.Services.ConfigureHttpJsonOptions(options =>
{
    options.SerializerOptions.TypeInfoResolverChain.Insert(0, AppJsonSerializerContext.Default);
});

var app = builder.Build();

app.MapGet("/health", () => CheckHealthQueryHandler.HandleAsync(new CheckHealthQuery(), CancellationToken.None));

var v1 = app.MapGroup("/v1");
v1.MapGet("/health", () => CheckHealthQueryHandler.HandleAsync(new CheckHealthQuery(), CancellationToken.None));

await app.RunAsync().ConfigureAwait(false);
