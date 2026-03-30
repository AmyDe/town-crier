namespace TownCrier.IntegrationTests;

internal static class IntegrationTestConfig
{
    public static string ApiBaseUrl =>
        GetRequired("INTEGRATION_TEST_API_BASE_URL");

    private static string GetRequired(string name) =>
        Environment.GetEnvironmentVariable(name)
            ?? throw new InvalidOperationException(
                $"Required environment variable '{name}' is not set.");
}
