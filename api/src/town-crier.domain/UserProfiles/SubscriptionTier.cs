using System.Text.Json.Serialization;

namespace TownCrier.Domain.UserProfiles;

[JsonConverter(typeof(JsonStringEnumConverter<SubscriptionTier>))]
public enum SubscriptionTier
{
    Free,
    Personal,
    Pro,
}
