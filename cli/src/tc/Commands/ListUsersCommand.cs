using System.Globalization;
using System.Text;
using Tc.Json;

namespace Tc.Commands;

internal static class ListUsersCommand
{
    public static async Task<int> RunAsync(ApiClient client, ParsedArgs args, CancellationToken ct)
    {
        var search = args.GetOptional("search");
        var pageSizeStr = args.GetOptional("page-size");
        var pageSize = 20;

        if (pageSizeStr is not null
            && (!int.TryParse(pageSizeStr, NumberStyles.None, CultureInfo.InvariantCulture, out pageSize)
                || pageSize <= 0))
        {
            await Console.Error.WriteLineAsync("Invalid --page-size: must be a positive integer")
                .ConfigureAwait(false);
            return 1;
        }

        string? continuationToken = null;

        do
        {
            var path = BuildPath(search, pageSize, continuationToken);

            ListUsersResponse? response;
            try
            {
                response = await client.GetFromJsonAsync(path, TcJsonContext.Default.ListUsersResponse, ct)
                    .ConfigureAwait(false);
            }
            catch (HttpRequestException ex)
            {
                await Console.Error.WriteLineAsync(ex.Message).ConfigureAwait(false);
                return 2;
            }

            if (response is null)
            {
                await Console.Error.WriteLineAsync("Empty response from API").ConfigureAwait(false);
                return 2;
            }

            PrintTable(response);
            continuationToken = response.ContinuationToken;

            if (continuationToken is null)
            {
                break;
            }

            await Console.Out.WriteAsync("Next page? [y/N] ").ConfigureAwait(false);
            var input = Console.ReadLine();
            if (!string.Equals(input?.Trim(), "y", StringComparison.OrdinalIgnoreCase))
            {
                break;
            }
        }
        while (true);

        return 0;
    }

    private static string BuildPath(string? search, int pageSize, string? continuationToken)
    {
        var sb = new StringBuilder("/v1/admin/users?pageSize=");
        sb.Append(pageSize.ToString(CultureInfo.InvariantCulture));

        if (search is not null)
        {
            sb.Append("&search=");
            sb.Append(Uri.EscapeDataString(search));
        }

        if (continuationToken is not null)
        {
            sb.Append("&continuationToken=");
            sb.Append(Uri.EscapeDataString(continuationToken));
        }

        return sb.ToString();
    }

    private static void PrintTable(ListUsersResponse response)
    {
        Console.WriteLine($"{"UserId",-24} {"Email",-32} {"Tier",-10}");
        Console.WriteLine(new string('-', 66));

        foreach (var item in response.Items)
        {
            Console.WriteLine(
                $"{item.UserId,-24} {item.Email ?? "(none)",-32} {item.Tier,-10}");
        }
    }
}
