using System.Globalization;
using System.Runtime.CompilerServices;
using System.Text.Json;
using TownCrier.Application.PlanIt;
using TownCrier.Domain.PlanningApplications;

namespace TownCrier.Infrastructure.PlanIt;

public sealed class PlanItClient(HttpClient httpClient) : IPlanItClient
{
    private const int DefaultPageSize = 5000;

    public async IAsyncEnumerable<PlanningApplication> FetchApplicationsAsync(
        DateTimeOffset? differentStart,
        [EnumeratorCancellation] CancellationToken ct)
    {
        var page = 1;
        int fetched;

        do
        {
            var url = new Uri(BuildUrl(differentStart, page), UriKind.Relative);
            using var response = await httpClient.GetAsync(url, ct).ConfigureAwait(false);
            response.EnsureSuccessStatusCode();

            var planItResponse = await JsonSerializer.DeserializeAsync(
                await response.Content.ReadAsStreamAsync(ct).ConfigureAwait(false),
                PlanItJsonSerializerContext.Default.PlanItResponse,
                ct).ConfigureAwait(false);

            if (planItResponse is null)
            {
                yield break;
            }

            fetched = planItResponse.Records.Count;

            foreach (var record in planItResponse.Records)
            {
                yield return MapToDomain(record);
            }

            page++;
        }
        while (fetched >= DefaultPageSize);
    }

    private static string BuildUrl(DateTimeOffset? differentStart, int page)
    {
        var url = $"/api/applics/json?pg_sz={DefaultPageSize}&sort=-last_different&page={page}";

        if (differentStart.HasValue)
        {
            url += $"&different_start={differentStart.Value:yyyy-MM-ddTHH:mm:ss}";
        }

        return url;
    }

    private static PlanningApplication MapToDomain(PlanItApplicationRecord record)
    {
        return new PlanningApplication(
            name: record.Name,
            uid: record.Uid,
            areaName: record.AreaName,
            areaId: record.AreaId,
            address: record.Address,
            postcode: record.Postcode,
            description: record.Description,
            appType: record.AppType,
            appState: record.AppState,
            appSize: record.AppSize,
            startDate: ParseDate(record.StartDate),
            decidedDate: ParseDate(record.DecidedDate),
            consultedDate: ParseDate(record.ConsultedDate),
            longitude: record.LocationX,
            latitude: record.LocationY,
            url: record.Url,
            link: record.Link,
            lastDifferent: DateTimeOffset.Parse(record.LastDifferent, CultureInfo.InvariantCulture));
    }

    private static DateOnly? ParseDate(string? value)
    {
        if (string.IsNullOrEmpty(value))
        {
            return null;
        }

        return DateOnly.Parse(value, CultureInfo.InvariantCulture);
    }
}
