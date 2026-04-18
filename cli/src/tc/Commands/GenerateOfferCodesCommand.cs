using System.Globalization;
using Tc.Json;

namespace Tc.Commands;

internal static class GenerateOfferCodesCommand
{
    private const int MinCount = 1;
    private const int MaxCount = 1000;
    private const int MinDurationDays = 1;
    private const int MaxDurationDays = 365;

    private static readonly HashSet<string> ValidTiers = new(StringComparer.OrdinalIgnoreCase)
    {
        "Personal", "Pro",
    };

    public static async Task<int> RunAsync(ApiClient client, ParsedArgs args, CancellationToken ct)
    {
        const string usage =
            "Usage: tc generate-offer-codes --count <N> --tier <Personal|Pro> --duration-days <D>";

        string countStr;
        try
        {
            countStr = args.GetRequired("count");
        }
        catch (ArgumentException)
        {
            await Console.Error.WriteLineAsync("Missing required argument: --count").ConfigureAwait(false);
            await Console.Error.WriteLineAsync(usage).ConfigureAwait(false);
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
            await Console.Error.WriteLineAsync(usage).ConfigureAwait(false);
            return 1;
        }

        string durationDaysStr;
        try
        {
            durationDaysStr = args.GetRequired("duration-days");
        }
        catch (ArgumentException)
        {
            await Console.Error.WriteLineAsync("Missing required argument: --duration-days").ConfigureAwait(false);
            await Console.Error.WriteLineAsync(usage).ConfigureAwait(false);
            return 1;
        }

        if (!int.TryParse(countStr, NumberStyles.None, CultureInfo.InvariantCulture, out var count)
            || count < MinCount
            || count > MaxCount)
        {
            await Console.Error.WriteLineAsync(
                $"Invalid --count: must be an integer between {MinCount} and {MaxCount}").ConfigureAwait(false);
            return 1;
        }

        if (!ValidTiers.Contains(tier))
        {
            await Console.Error.WriteLineAsync(
                $"Invalid tier: {tier}. Must be one of: Personal, Pro").ConfigureAwait(false);
            return 1;
        }

        // Normalise tier casing to match API expectation.
        tier = ValidTiers.First(t => string.Equals(t, tier, StringComparison.OrdinalIgnoreCase));

        if (!int.TryParse(durationDaysStr, NumberStyles.None, CultureInfo.InvariantCulture, out var durationDays)
            || durationDays < MinDurationDays
            || durationDays > MaxDurationDays)
        {
            await Console.Error.WriteLineAsync(
                $"Invalid --duration-days: must be an integer between {MinDurationDays} and {MaxDurationDays}")
                .ConfigureAwait(false);
            return 1;
        }

        var request = new GenerateOfferCodesRequest
        {
            Count = count,
            Tier = tier,
            DurationDays = durationDays,
        };

        HttpResponseMessage response;
        try
        {
            response = await client.PostAsJsonAsync(
                "/v1/admin/offer-codes",
                request,
                TcJsonContext.Default.GenerateOfferCodesRequest,
                ct).ConfigureAwait(false);
        }
        catch (HttpRequestException ex)
        {
            await Console.Error.WriteLineAsync($"API error: {ex.Message}").ConfigureAwait(false);
            return 2;
        }

        try
        {
            if (!response.IsSuccessStatusCode)
            {
                var body = await response.Content.ReadAsStringAsync(ct).ConfigureAwait(false);
                await Console.Error.WriteLineAsync(
                    $"API error ({(int)response.StatusCode}): {body}").ConfigureAwait(false);
                return 2;
            }

            var stream = await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false);
            await using var streamScope = stream.ConfigureAwait(false);
            using var reader = new StreamReader(stream);

            while (await reader.ReadLineAsync(ct).ConfigureAwait(false) is { } line)
            {
                if (line.Length == 0)
                {
                    continue;
                }

                await Console.Out.WriteLineAsync(line).ConfigureAwait(false);
            }
        }
        finally
        {
            response.Dispose();
        }

        await Console.Error.WriteLineAsync(
            $"Generated {count} codes: {tier} tier, {durationDays} days duration").ConfigureAwait(false);
        return 0;
    }
}
