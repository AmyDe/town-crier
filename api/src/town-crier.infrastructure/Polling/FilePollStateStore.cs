using System.Globalization;
using TownCrier.Application.Polling;

namespace TownCrier.Infrastructure.Polling;

public sealed class FilePollStateStore : IPollStateStore
{
    private readonly string filePath;

    public FilePollStateStore(string filePath)
    {
        this.filePath = filePath;
    }

    public async Task<DateTimeOffset?> GetLastPollTimeAsync(CancellationToken ct)
    {
        if (!File.Exists(this.filePath))
        {
            return null;
        }

        var content = await File.ReadAllTextAsync(this.filePath, ct).ConfigureAwait(false);

        if (string.IsNullOrWhiteSpace(content))
        {
            return null;
        }

        return DateTimeOffset.Parse(content.Trim(), CultureInfo.InvariantCulture);
    }

    public async Task SaveLastPollTimeAsync(DateTimeOffset pollTime, CancellationToken ct)
    {
        var directory = Path.GetDirectoryName(this.filePath);
        if (!string.IsNullOrEmpty(directory))
        {
            Directory.CreateDirectory(directory);
        }

        await File.WriteAllTextAsync(
            this.filePath,
            pollTime.ToString("O", CultureInfo.InvariantCulture),
            ct).ConfigureAwait(false);
    }
}
