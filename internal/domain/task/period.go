package task

import (
	"fmt"
	"time"
)

type PeriodType string

const (
	PeriodTypeDaily         PeriodType = "daily"
	PeriodTypeMonthly       PeriodType = "monthly"
	PeriodTypeSpecificDates PeriodType = "specific_dates"
	PeriodTypeEvenOdd       PeriodType = "even_odd"
)

type Parity string

const (
	ParityEven Parity = "even"
	ParityOdd  Parity = "odd"
)

type Period struct {
	Type       PeriodType
	Interval   int
	DayOfMonth int
	Dates      []time.Time
	Parity     Parity
}

func (p Period) Valid() error {
	switch p.Type {
	case PeriodTypeDaily:
		if p.Interval < 1 {
			return fmt.Errorf("daily: interval must be >= 1")
		}
	case PeriodTypeMonthly:
		if p.DayOfMonth < 1 || p.DayOfMonth > 30 {
			return fmt.Errorf("monthly: day_of_month must be between 1 and 30")
		}
	case PeriodTypeSpecificDates:
		if len(p.Dates) == 0 {
			return fmt.Errorf("specific_dates: at least one date is required")
		}
	case PeriodTypeEvenOdd:
		if p.Parity != ParityEven && p.Parity != ParityOdd {
			return fmt.Errorf("even_odd: parity must be 'even' or 'odd'")
		}
	default:
		return fmt.Errorf("unknown period type: %q", p.Type)
	}

	return nil
}

func (p Period) Compute(from, to, anchor time.Time) []time.Time {
	from = TruncateToDate(from)
	to = TruncateToDate(to)
	anchor = TruncateToDate(anchor)

	switch p.Type {
	case PeriodTypeDaily:
		return computeDaily(from, to, anchor, p.Interval)
	case PeriodTypeMonthly:
		return computeMonthly(from, to, p.DayOfMonth)
	case PeriodTypeSpecificDates:
		return computeSpecificDates(from, to, p.Dates)
	case PeriodTypeEvenOdd:
		return computeEvenOdd(from, to, p.Parity)
	default:
		return nil
	}
}

func computeDaily(from, to, anchor time.Time, interval int) []time.Time {
	daysDiff := int(from.Sub(anchor).Hours() / 24)

	rem := daysDiff % interval
	if rem < 0 {
		rem += interval
	}

	var firstAligned time.Time
	if rem == 0 {
		firstAligned = from
	} else {
		firstAligned = from.AddDate(0, 0, interval-rem)
	}

	var dates []time.Time
	for d := firstAligned; !d.After(to); d = d.AddDate(0, 0, interval) {
		dates = append(dates, d)
	}

	return dates
}

func computeMonthly(from, to time.Time, dayOfMonth int) []time.Time {
	var dates []time.Time

	cur := time.Date(from.Year(), from.Month(), dayOfMonth, 0, 0, 0, 0, time.UTC)
	if cur.Before(from) {
		cur = time.Date(cur.Year(), cur.Month()+1, dayOfMonth, 0, 0, 0, 0, time.UTC)
	}

	for !cur.After(to) {
		if cur.Day() == dayOfMonth {
			dates = append(dates, cur)
		}
		cur = time.Date(cur.Year(), cur.Month()+1, dayOfMonth, 0, 0, 0, 0, time.UTC)
	}

	return dates
}

func computeSpecificDates(from, to time.Time, dates []time.Time) []time.Time {
	var result []time.Time
	for _, d := range dates {
		d = TruncateToDate(d)
		if !d.Before(from) && !d.After(to) {
			result = append(result, d)
		}
	}

	return result
}

func computeEvenOdd(from, to time.Time, parity Parity) []time.Time {
	var dates []time.Time
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		day := d.Day()
		if parity == ParityEven && day%2 == 0 {
			dates = append(dates, d)
		} else if parity == ParityOdd && day%2 != 0 {
			dates = append(dates, d)
		}
	}

	return dates
}

func TruncateToDate(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
