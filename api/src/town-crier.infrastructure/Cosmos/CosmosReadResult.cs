namespace TownCrier.Infrastructure.Cosmos;

/// <summary>
/// Result of a document read that surfaces the Cosmos-assigned ETag.
/// <see cref="Document"/> is <c>null</c> when the document does not exist
/// (HTTP 404). <see cref="ETag"/> is <c>null</c> in the same case.
/// </summary>
/// <typeparam name="T">The deserialized document type.</typeparam>
/// <param name="Document">The deserialized document, or <c>null</c> when the document was not found.</param>
/// <param name="ETag">The Cosmos-assigned ETag value (quoted, as required by If-Match), or <c>null</c> when not found.</param>
public sealed record CosmosReadResult<T>(T? Document, string? ETag);
