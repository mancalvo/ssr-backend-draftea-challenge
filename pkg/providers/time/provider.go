package time

import stdtime "time"

// SystemProvider provides the current system time.
type SystemProvider struct{}

func (p *SystemProvider) Now() stdtime.Time {
	return stdtime.Now()
}
