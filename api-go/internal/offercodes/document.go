package offercodes

import (
	"time"

	"github.com/AmyDe/town-crier/api-go/internal/platform"
	"github.com/AmyDe/town-crier/api-go/internal/profiles"
)

// offerCodeDocument is the Cosmos persistence shape, mirroring the .NET
// OfferCodeDocument field-for-field (camelCase, .NET DateTimeOffset format for
// the timestamps). id == code == partition key.
type offerCodeDocument struct {
	ID               string               `json:"id"`
	Code             string               `json:"code"`
	Tier             string               `json:"tier"`
	DurationDays     int                  `json:"durationDays"`
	CreatedAt        platform.DotNetTime  `json:"createdAt"`
	RedeemedByUserID *string              `json:"redeemedByUserId"`
	RedeemedAt       *platform.DotNetTime `json:"redeemedAt"`
}

func newOfferCodeDocument(c OfferCode) offerCodeDocument {
	doc := offerCodeDocument{
		ID:               c.Code,
		Code:             c.Code,
		Tier:             c.Tier.String(),
		DurationDays:     c.DurationDays,
		CreatedAt:        platform.DotNetTime(c.CreatedAt),
		RedeemedByUserID: c.RedeemedByUserID,
	}
	if c.RedeemedAt != nil {
		at := platform.DotNetTime(*c.RedeemedAt)
		doc.RedeemedAt = &at
	}
	return doc
}

func (d offerCodeDocument) toDomain() (OfferCode, error) {
	tier, err := profiles.ParseSubscriptionTier(d.Tier)
	if err != nil {
		return OfferCode{}, err
	}
	c := OfferCode{
		Code:             d.Code,
		Tier:             tier,
		DurationDays:     d.DurationDays,
		CreatedAt:        time.Time(d.CreatedAt),
		RedeemedByUserID: d.RedeemedByUserID,
	}
	if d.RedeemedAt != nil {
		at := time.Time(*d.RedeemedAt)
		c.RedeemedAt = &at
	}
	return c, nil
}
