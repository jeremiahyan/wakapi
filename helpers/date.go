package helpers

import (
	"fmt"
	"github.com/muety/wakapi/config"
	"time"
)

// ParseDateTimeTZ attempts to parse the given date string from multiple formats.
// First, a time-zoned date-time string (e.g. 2006-01-02T15:04:05+02:00) is tried
// Second, a non-time-zoned date-time string (e.g. 2006-01-02 15:04:05) is tried at the given zone
// Third, a non-time-zoned date string (e.g. 2006-01-02) is tried at the given zone
// Example:
// - Server runs in CEST (UTC+2), requesting user lives in PDT (UTC-7).
// - 2021-04-25T10:30:00Z, 2021-04-25T3:30:00-0100 and 2021-04-25T12:30:00+0200 are equivalent, they represent the same point in time
// - When user requests non-time-zoned range (e.g. 2021-04-25T00:00:00), but has their time zone properly configured, this will resolve to 2021-04-25T09:00:00
func ParseDateTimeTZ(date string, tz *time.Location) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, date); err == nil {
		return t, nil
	}
	if t, err := time.ParseInLocation(config.SimpleDateTimeFormat, date, tz); err == nil {
		return t, nil
	}
	return time.ParseInLocation(config.SimpleDateFormat, date, tz)
}

func FormatDate(date time.Time) string {
	return date.Format(config.SimpleDateFormat)
}

func FormatDateTime(date time.Time) string {
	return date.Format(config.SimpleDateTimeFormat)
}

func FormatDateTimeHuman(date time.Time) string {
	return date.Format(config.Get().App.DateTimeFormat)
}

func FormatDateHuman(date time.Time) string {
	return date.Format(config.Get().App.DateFormat)
}

func FmtWakatimeDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	return fmt.Sprintf("%d hrs %d mins", h, m)
}
