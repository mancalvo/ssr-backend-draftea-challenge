package time

import stdtime "time"

type Provider interface {
	Now() stdtime.Time
}

type SystemProvider struct{}

func (p *SystemProvider) Now() stdtime.Time {
	return stdtime.Now()
}
