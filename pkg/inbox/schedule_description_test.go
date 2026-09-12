package inbox

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDescribeSchedule(t *testing.T) {
	for _, tt := range []struct{ expression, zone, want string }{
		{"0 * * * * *", "", "Every minute (UTC)"},
		{"0 */15 * * * *", "UTC", "Every 15 minutes (UTC)"},
		{"0 0 5 * * *", "America/Los_Angeles", "Daily at 05:00 (America/Los_Angeles)"},
		{"0 30 6 * * *", "UTC", "Daily at 06:30 (UTC)"},
		{"0 30 9 * * 1-5", "UTC", "At 09:30 on Monday, Tuesday, Wednesday, Thursday, Friday (UTC)"},
		{"0 0 6 1 1 *", "UTC", "At 06:00 on day of month 1 in January (UTC)"},
	} {
		t.Run(tt.expression, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			got, err := describeSchedule(tt.expression, tt.zone)
			r.NoError(err)
			a.Equal(tt.want, got)
		})
	}
	for _, expression := range []string{"invalid", "0 90 6 * * *"} {
		_, err := describeSchedule(expression, "UTC")
		require.Error(t, err)
	}
}
