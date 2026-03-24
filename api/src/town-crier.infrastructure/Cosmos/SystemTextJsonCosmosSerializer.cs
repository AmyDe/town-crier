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
        if (typeof(Stream).IsAssignableFrom(typeof(T)))
        {
            return (T)(object)stream;
        }

        using (stream)
        {
            return (T)JsonSerializer.Deserialize(stream, this.options.GetTypeInfo(typeof(T)))!;
        }
    }

    public override Stream ToStream<T>(T input)
    {
        var stream = new MemoryStream();
        JsonSerializer.Serialize(stream, input, this.options.GetTypeInfo(typeof(T)));
        stream.Position = 0;
        return stream;
    }
}
