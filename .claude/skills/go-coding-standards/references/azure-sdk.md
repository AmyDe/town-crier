# Data Access & Azure SDK (reference)

Read when the bead touches a store, persistence, Cosmos DB, Service Bus, Azure Communication Services, Azure auth/credentials, or partition-key design. The core (`SKILL.md`) states the no-ORM rule; this file is the full detail.

## 11. Data access — official Azure SDK

- **Cosmos DB**: `github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos`. Official Microsoft SDK, actively maintained. Do **not** use `microsoft/gocosmos` (a `database/sql` driver that loses Cosmos semantics) or the community vippsas SDK.
- **Service Bus**: `github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus`. The official SDK is the only supported path — do not hand-roll a REST client.
- **Auth**: `github.com/Azure/azure-sdk-for-go/sdk/azidentity`. Use `DefaultAzureCredential` in deployed environments; `ClientSecretCredential` only where required.
- **Communication Services (email)**: `github.com/Azure/azure-sdk-for-go/sdk/messaging/azcommunicationservices/sender` (or the `azcommunication` namespace's email package — check the current name in `go.mod` when implementing).
- **Store struct holds the SDK client**, exposes only the methods the consumer interface declares; consumers never see SDK types.
- **Reuse the domain struct as the stored document when the shapes match.** Introduce a separate document struct only when the persistence shape genuinely diverges (partition-key field, `_etag`, denormalised fields) — not as default DTO-mapping ceremony.
- **No ORM**, no `gorm`, no `sqlx`. Cosmos is not relational.
- **Partition keys** are designed around query access patterns. Document the choice per container in a comment at the top of the store file.
