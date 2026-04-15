using Tc.Json;

namespace Tc.Commands;

internal static class GrantSubscriptionCommand
{
    private static readonly HashSet<string> ValidTiers = new(StringComparer.OrdinalIgnoreCase)
    {
        "Free", "Personal", "Pro",
    };

    public static async Task<int> RunAsync(ApiClient client, ParsedArgs args, CancellationToken ct)
    {
        string email;
        try
        {
            email = args.GetRequired("email");
        }
        catch (ArgumentException)
        {
            await Console.Error.WriteLineAsync("Missing required argument: --email").ConfigureAwait(false);
            await Console.Error.WriteLineAsync("Usage: tc grant-subscription --email <email> --tier <Free|Personal|Pro>").ConfigureAwait(false);
            return 1;
        }

        string tier;
        try
        {
            tier = args.GetRequired("tier");
        }
        catch (ArgumentException)
        {
            await Console.Error.WriteLineAsync("Missing required argument: --tier").ConfigureAwait(false);
            await Console.Error.WriteLineAsync("Usage: tc grant-subscription --email <email> --tier <Free|Personal|Pro>").ConfigureAwait(false);
            return 1;
        }

        if (!ValidTiers.Contains(tier))
        {
            await Console.Error.WriteLineAsync($"Invalid tier: {tier}. Must be one of: Free, Personal, Pro").ConfigureAwait(false);
            return 1;
        }

        // Normalise tier casing to match API expectation
        tier = ValidTiers.First(t => string.Equals(t, tier, StringComparison.OrdinalIgnoreCase));

        var request = new GrantSubscriptionRequest
        {
            Email = email,
            Tier = tier,
        };

        var response = await client.PutAsJsonAsync(
            "/v1/admin/subscriptions",
            request,
            TcJsonContext.Default.GrantSubscriptionRequest,
            ct).ConfigureAwait(false);

        if (response.StatusCode == System.Net.HttpStatusCode.NotFound)
        {
            await Console.Error.WriteLineAsync($"User not found: {email}").ConfigureAwait(false);
            return 2;
        }

        if (!response.IsSuccessStatusCode)
        {
            var body = await response.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
            await Console.Error.WriteLineAsync($"API error ({(int)response.StatusCode}): {body}").ConfigureAwait(false);
            return 2;
        }

        await Console.Out.WriteLineAsync($"Subscription granted: {email} -> {tier}").ConfigureAwait(false);
        return 0;
    }
}
