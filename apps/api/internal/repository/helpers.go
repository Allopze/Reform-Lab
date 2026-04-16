package repository

import (
	"time"

	"github.com/google/uuid"
)

const timeLayout = time.RFC3339Nano

func parseTime(s string) (time.Time, error) {
	return time.Parse(timeLayout, s)
}

func timePtr(s *string) *time.Time {
	if s == nil || *s == "" {
		return nil
	}
	t, err := parseTime(*s)
	if err != nil {
		return nil
	}
	return &t
}

func fmtTimePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(timeLayout)
	return &s
}

func nullableUUIDString(id *uuid.UUID) *string {
	if id == nil {
		return nil
	}
	s := id.String()
	return &s
}

func nullableString(value *string) interface{} {
	if value == nil {
		return nil
	}
	return *value
}
