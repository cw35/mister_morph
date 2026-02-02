package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type cronExpr struct {
	minute *valueSet
	hour   *valueSet
	dom    *valueSet
	month  *valueSet
	dow    *valueSet

	domAny bool
	dowAny bool
}

func parseCronExpr(expr string) (*cronExpr, error) {
	fields := strings.Fields(strings.TrimSpace(expr))
	if len(fields) != 5 {
		return nil, fmt.Errorf("invalid cron expression (expected 5 fields): %q", expr)
	}

	min, err := parseField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minute: %w", err)
	}
	hour, err := parseField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hour: %w", err)
	}
	dom, domAny, err := parseFieldWithAny(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("dom: %w", err)
	}
	month, err := parseField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("month: %w", err)
	}
	dow, dowAny, err := parseFieldWithAny(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("dow: %w", err)
	}

	return &cronExpr{
		minute: min,
		hour:   hour,
		dom:    dom,
		month:  month,
		dow:    dow,
		domAny: domAny,
		dowAny: dowAny,
	}, nil
}

// next returns the next matching time strictly after "after", searching up to 366 days.
func (e *cronExpr) next(after time.Time) (time.Time, error) {
	start := after.UTC().Add(time.Minute).Truncate(time.Minute)
	limit := start.Add(366 * 24 * time.Hour)
	for t := start; t.Before(limit); t = t.Add(time.Minute) {
		if !e.minute.has(t.Minute()) {
			continue
		}
		if !e.hour.has(t.Hour()) {
			continue
		}
		if !e.month.has(int(t.Month())) {
			continue
		}
		domMatch := e.dom.has(t.Day())
		dowMatch := e.dow.has(int(t.Weekday()))

		// Standard cron semantics: if both DOM and DOW are restricted (not "*"),
		// treat them as OR; otherwise require the non-any field.
		if !e.domAny && !e.dowAny {
			if !(domMatch || dowMatch) {
				continue
			}
		} else {
			if !e.domAny && !domMatch {
				continue
			}
			if !e.dowAny && !dowMatch {
				continue
			}
		}
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("no matching time within search window")
}

type valueSet struct {
	min int
	max int
	all bool
	val map[int]struct{}
}

func (s *valueSet) has(v int) bool {
	if s == nil {
		return false
	}
	if s.all {
		return true
	}
	_, ok := s.val[v]
	return ok
}

func parseFieldWithAny(tok string, min, max int) (*valueSet, bool, error) {
	vs, err := parseField(tok, min, max)
	if err != nil {
		return nil, false, err
	}
	return vs, strings.TrimSpace(tok) == "*", nil
}

func parseField(tok string, min, max int) (*valueSet, error) {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return nil, fmt.Errorf("empty field")
	}
	if tok == "*" {
		return &valueSet{min: min, max: max, all: true, val: nil}, nil
	}

	out := &valueSet{min: min, max: max, all: false, val: make(map[int]struct{})}
	parts := strings.Split(tok, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "*" {
			out.all = true
			out.val = nil
			return out, nil
		}
		if strings.HasPrefix(p, "*/") {
			stepStr := strings.TrimPrefix(p, "*/")
			step, err := strconv.Atoi(stepStr)
			if err != nil || step <= 0 {
				return nil, fmt.Errorf("invalid step %q", p)
			}
			for v := min; v <= max; v += step {
				out.val[v] = struct{}{}
			}
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid value %q", p)
		}
		if n < min || n > max {
			return nil, fmt.Errorf("value %d out of range (%d-%d)", n, min, max)
		}
		out.val[n] = struct{}{}
	}
	if out.all {
		out.val = nil
	}
	if !out.all && len(out.val) == 0 {
		return nil, fmt.Errorf("no values parsed from %q", tok)
	}
	return out, nil
}
