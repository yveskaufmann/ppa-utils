package distinfo

import (
	"testing"
	"time"
)

func TestDistributionUsesConcreteDateFields(t *testing.T) {
	d := Distribution{
		CreatedAt:     time.Unix(1, 0).UTC(),
		ReleasedAt:    time.Unix(2, 0).UTC(),
		EOLAt:         time.Unix(3, 0).UTC(),
		EOLEsmDate:    time.Unix(4, 0).UTC(),
		EOLLegacyDate: time.Unix(5, 0).UTC(),
	}

	if d.CreatedAt.IsZero() || d.ReleasedAt.IsZero() || d.EOLAt.IsZero() || d.EOLEsmDate.IsZero() || d.EOLLegacyDate.IsZero() {
		t.Fatal("all concrete date fields should be set")
	}
}
