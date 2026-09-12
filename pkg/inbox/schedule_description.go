package inbox

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

func describeSchedule(expression, zone string) (string, error) {
	if zone == "" {
		zone = "UTC"
	}
	if _, err := time.LoadLocation(zone); err != nil {
		return "", err
	}
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	parsed, err := parser.Parse(expression)
	if err != nil {
		return "", err
	}
	spec := parsed.(*cron.SpecSchedule)
	fields := strings.Fields(expression)
	label := ""
	if fields[0] == "0" && fields[1] == "*" && fields[2] == "*" {
		label = "Every minute"
	} else if fields[0] == "0" && strings.HasPrefix(fields[1], "*/") && fields[2] == "*" {
		label = "Every " + strings.TrimPrefix(fields[1], "*/") + " minutes"
	} else if hour, e := strconv.Atoi(fields[2]); e == nil {
		if minute, e := strconv.Atoi(fields[1]); e == nil && fields[0] == "0" {
			label = fmt.Sprintf("Daily at %02d:%02d", hour, minute)
		}
	}
	if label == "" {
		label = "At second " + cronValues(spec.Second, 0, 59, nil) + "; minute " + cronValues(spec.Minute, 0, 59, nil) + "; hour " + cronValues(spec.Hour, 0, 23, nil)
	}
	if fields[3] != "*" || fields[5] != "*" {
		label = strings.TrimPrefix(label, "Daily ")
		if strings.HasPrefix(label, "at ") {
			label = "At " + strings.TrimPrefix(label, "at ")
		}
		restrictions := []string{}
		if fields[3] != "*" {
			restrictions = append(restrictions, "day of month "+cronValues(spec.Dom, 1, 31, nil))
		}
		if fields[5] != "*" {
			restrictions = append(restrictions, cronValues(spec.Dow, 0, 6, []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}))
		}
		label += " on " + strings.Join(restrictions, " or ")
	}
	if fields[4] != "*" {
		label += " in " + cronValues(spec.Month, 1, 12, []string{"", "January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"})
	}
	return label + " (" + zone + ")", nil
}

func cronValues(bits uint64, min, max int, names []string) string {
	values := []string{}
	for i := min; i <= max; i++ {
		if bits&(1<<uint(i)) != 0 {
			value := strconv.Itoa(i)
			if names != nil {
				value = names[i]
			}
			values = append(values, value)
		}
	}
	if len(values) == max-min+1 {
		return "every"
	}
	return strings.Join(values, ", ")
}
