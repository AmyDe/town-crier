using System.Text.Json;
using Microsoft.Azure.Cosmos;

namespace TownCrier.Infrastructure.Cosmos;

public sealed class SystemTextJsonCosmosSerializer : CosmosSerializer
{
    private readonly JsonSerializerOptions options;

    public SystemTextJsonCosmosSerializer(JsonSerializerOptions options)
    {
        ArgumentNullException.ThrowIfNull(options);
        this.options = options;
    }

    public override T FromStream<T>(Stream stream)
    {
        using (stream)
        {
            if (typeof(Stream).IsAssignableFrom(typeof(T)))
            {
                return (T)(object)stream;
            }

            return JsonSerializer.Deserialize<T>(stream, this.options)!;
        }
    }

    public override Stream ToStream<T>(T input)
    {
        var stream = new MemoryStream();
        JsonSerializer.Serialize(stream, input, this.options);
        stream.Position = 0;
        return stream;
    }
}
