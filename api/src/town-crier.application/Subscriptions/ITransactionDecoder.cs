namespace TownCrier.Application.Subscriptions;

/// <summary>
/// Decodes a raw JSON payload (from a verified JWS) into a <see cref="DecodedTransaction"/>.
/// Implementation lives in the infrastructure layer with System.Text.Json source generators.
/// </summary>
public interface ITransactionDecoder
{
    DecodedTransaction Decode(string json);
}
