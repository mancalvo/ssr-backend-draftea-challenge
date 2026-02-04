package idgen

import "github.com/rs/xid"

type Generator interface {
	Generate() string
}

type XIDGenerator struct{}

func (g *XIDGenerator) Generate() string {
	return xid.New().String()
}
