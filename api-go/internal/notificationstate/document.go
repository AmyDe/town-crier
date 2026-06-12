package notificationstate

import (
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
)

// stateDocument is the stored Cosmos shape, byte-compatible with the .NET
// NotificationStateDocument: camelCase keys, userId doubling as id and
// partition key (one document per user), lastReadAt in the .NET
// DateTimeOffset wire format.
type stateDocument struct {
	ID         string              `json:"id"`
	UserID     string              `json:"userId"`
	LastReadAt platform.DotNetTime `json:"lastReadAt"`
	Version    int                 `json:"version"`
}

func newStateDocument(s State) stateDocument {
	return stateDocument{
		ID:         s.UserID,
		UserID:     s.UserID,
		LastReadAt: platform.DotNetTime(s.LastReadAt),
		Version:    s.Version,
	}
}

func (d stateDocument) toDomain() State {
	return State{
		UserID:     d.UserID,
		LastReadAt: time.Time(d.LastReadAt),
		Version:    d.Version,
	}
}
