package idgen

import "github.com/rs/xid"

// XIDGenerator generates unique XIDs.
type XIDGenerator struct{}

func (g *XIDGenerator) Generate() string {
	return xid.New().String()
}
