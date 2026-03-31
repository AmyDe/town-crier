namespace TownCrier.IntegrationTests;

internal static class IntegrationTestConfig
{
    private static readonly Dictionary<string, string> FileOverrides = LoadEnvFile();
    private static Dictionary<string, string>? savedOverrides;

    public static string ApiBaseUrl =>
        GetRequired("INTEGRATION_TEST_API_BASE_URL");

    public static string Auth0Domain =>
        GetRequired("INTEGRATION_TEST_AUTH0_DOMAIN");

    public static string Auth0ClientId =>
        GetRequired("INTEGRATION_TEST_AUTH0_CLIENT_ID");

    public static string? Auth0ClientSecret =>
        GetValue("INTEGRATION_TEST_AUTH0_CLIENT_SECRET");

    public static string Auth0Audience =>
        GetRequired("INTEGRATION_TEST_AUTH0_AUDIENCE");

    public static string Username =>
        GetRequired("INTEGRATION_TEST_USERNAME");

    public static string Password =>
        GetRequired("INTEGRATION_TEST_PASSWORD");

    internal static string? GetValue(string name)
    {
        var env = Environment.GetEnvironmentVariable(name);
        if (!string.IsNullOrEmpty(env))
        {
            return env;
        }

        return FileOverrides.TryGetValue(name, out var value)
            && !string.IsNullOrEmpty(value)
                ? value
                : null;
    }

    internal static void ResetFileOverrides()
    {
        savedOverrides = new Dictionary<string, string>(FileOverrides);
        FileOverrides.Clear();
    }

    internal static void RestoreFileOverrides()
    {
        if (savedOverrides is null)
        {
            return;
        }

        foreach (var (key, value) in savedOverrides)
        {
            FileOverrides[key] = value;
        }

        savedOverrides = null;
    }

    private static string GetRequired(string name) =>
        GetValue(name)
            ?? throw new InvalidOperationException(
                $"Required environment variable '{name}' is not set.");

    /// <summary>
    /// Loads env vars from a .env file as a fallback for environments where
    /// .NET does not inherit process environment variables (observed with
    /// .NET 10 on GitHub Actions runners).
    /// </summary>
    private static Dictionary<string, string> LoadEnvFile()
    {
        var result = new Dictionary<string, string>();
        var envFile = Path.Combine(AppContext.BaseDirectory, ".env");
        if (!File.Exists(envFile))
        {
            envFile = Path.Combine(Directory.GetCurrentDirectory(), ".env");
        }

        if (!File.Exists(envFile))
        {
            return result;
        }

        foreach (var line in File.ReadAllLines(envFile))
        {
            var trimmed = line.Trim();
            if (string.IsNullOrEmpty(trimmed) || trimmed.StartsWith('#'))
            {
                continue;
            }

            var eqIndex = trimmed.IndexOf('=', StringComparison.Ordinal);
            if (eqIndex > 0)
            {
                var key = trimmed[..eqIndex];
                var val = trimmed[(eqIndex + 1)..];
                result[key] = val;
            }
        }

        return result;
    }
}
