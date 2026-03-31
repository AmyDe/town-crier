using System.Net.Http.Headers;
using TUnit.Core.Interfaces;

namespace TownCrier.IntegrationTests;

public sealed class ApiClientFixture : IAsyncInitializer, IAsyncDisposable
{
    private HttpClient? client;

    public HttpClient Client => this.client
        ?? throw new InvalidOperationException("Fixture not initialized.");

    public async Task InitializeAsync()
    {
        var token = await Auth0TokenProvider.GetTokenAsync().ConfigureAwait(false);

        this.client = new HttpClient
        {
            BaseAddress = new Uri(IntegrationTestConfig.ApiBaseUrl),
        };
        this.client.DefaultRequestHeaders.Authorization =
            new AuthenticationHeaderValue("Bearer", token);
    }

    public ValueTask DisposeAsync()
    {
        this.client?.Dispose();
        return ValueTask.CompletedTask;
    }
}
