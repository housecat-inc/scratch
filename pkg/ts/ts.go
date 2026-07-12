package ts

import (
	"database/sql/driver"
	"time"

	"github.com/cockroachdb/errors"
)

type Timestamp struct {
	time.Time
}

func Date(year int, month time.Month, day int) Timestamp {
	return New(time.Date(year, month, day, 0, 0, 0, 0, time.UTC))
}

func New(t time.Time) Timestamp {
	return Timestamp{Time: t.UTC()}
}

func NewPtr(t time.Time) *Timestamp {
	v := New(t)
	return &v
}

func Now() Timestamp {
	mu.RLock()
	defer mu.RUnlock()
	if mockTime != nil {
		return New(*mockTime)
	}
	return New(time.Now())
}

func NowPtr() *Timestamp {
	v := Now()
	return &v
}

func (ts *Timestamp) Scan(value any) error {
	if value == nil {
		ts.Time = time.Time{}
		return nil
	}
	switch v := value.(type) {
	case string:
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return errors.Wrap(err, "timestamp scan")
		}
		ts.Time = t.UTC()
		return nil
	case time.Time:
		ts.Time = v.UTC()
		return nil
	default:
		return errors.Newf("timestamp scan: unsupported type %T", value)
	}
}

func (ts Timestamp) Value() (driver.Value, error) {
	if ts.Time.IsZero() {
		return nil, nil
	}
	return ts.Time.UTC().Format(time.RFC3339), nil
}
