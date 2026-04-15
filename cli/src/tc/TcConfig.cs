using System.Text.Json;
using Tc.Json;

namespace Tc;

internal sealed class TcConfig
{
    public static string DefaultPath => Path.Combine(
        Environment.GetFolderPath(Environment.SpecialFolder.UserProfile),
        ".config",
        "tc",
        "config.json");

    public required string Url { get; init; }

    public required string ApiKey { get; init; }

    public static TcConfig Load(string path, string? url, string? apiKey)
    {
        var fileUrl = (string?)null;
        var fileApiKey = (string?)null;

        if (File.Exists(path))
        {
            var json = File.ReadAllText(path);
            var file = JsonSerializer.Deserialize(json, TcJsonContext.Default.ConfigFile);
            if (file is not null)
            {
                fileUrl = file.Url;
                fileApiKey = file.ApiKey;
            }
        }

        var resolvedUrl = url ?? fileUrl;
        var resolvedApiKey = apiKey ?? fileApiKey;

        if (string.IsNullOrEmpty(resolvedUrl))
        {
            throw new InvalidOperationException(
                $"API URL not configured. Set 'url' in {path} or pass --url.");
        }

        if (string.IsNullOrEmpty(resolvedApiKey))
        {
            throw new InvalidOperationException(
                $"API key not configured. Set 'apiKey' in {path} or pass --api-key.");
        }

        return new TcConfig
        {
            Url = resolvedUrl,
            ApiKey = resolvedApiKey,
        };
    }
}
