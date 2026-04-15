using System.Net.Http.Json;
using System.Text.Json.Serialization.Metadata;

namespace Tc;

internal sealed class ApiClient : IDisposable
{
    private const string ApiKeyHeader = "X-Admin-Key";

    private readonly HttpClient client;

    public ApiClient(TcConfig config)
    {
        this.client = new HttpClient
        {
            BaseAddress = new Uri(config.Url.TrimEnd('/')),
        };
        this.client.DefaultRequestHeaders.Add(ApiKeyHeader, config.ApiKey);
    }

    public async Task<HttpResponseMessage> PutAsJsonAsync<T>(
        string path,
        T body,
        JsonTypeInfo<T> typeInfo,
        CancellationToken ct)
    {
        return await this.client.PutAsJsonAsync(path, body, typeInfo, ct).ConfigureAwait(false);
    }

    public void Dispose()
    {
        this.client.Dispose();
    }
}
