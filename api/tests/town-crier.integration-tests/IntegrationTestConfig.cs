namespace TownCrier.IntegrationTests;

internal static class IntegrationTestConfig
{
    public static string ApiBaseUrl =>
        GetRequired("INTEGRATION_TEST_API_BASE_URL");

    public static string Auth0Domain =>
        GetRequired("INTEGRATION_TEST_AUTH0_DOMAIN");

    public static string Auth0ClientId =>
        GetRequired("INTEGRATION_TEST_AUTH0_CLIENT_ID");

    public static string? Auth0ClientSecret =>
        Environment.GetEnvironmentVariable("INTEGRATION_TEST_AUTH0_CLIENT_SECRET");

    public static string Auth0Audience =>
        GetRequired("INTEGRATION_TEST_AUTH0_AUDIENCE");

    public static string Username =>
        GetRequired("INTEGRATION_TEST_USERNAME");

    public static string Password =>
        GetRequired("INTEGRATION_TEST_PASSWORD");

    private static string GetRequired(string name) =>
        Environment.GetEnvironmentVariable(name)
            ?? throw new InvalidOperationException(
                $"Required environment variable '{name}' is not set.");
}
