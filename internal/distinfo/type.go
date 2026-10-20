package distinfo

import "time"

type Distribution struct {
	Version string
	Codename string
	Series string
	CreatedAt time.Time
	ReleasedAt time.Time
	EOLAt time.Time
	EOLEsmDate time.Time
	EOLLegacyDate time.Time
	IsLTS bool
}
