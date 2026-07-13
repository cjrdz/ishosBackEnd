// test_helpers_test.go — Shared test helpers for repository tests.
package repository

import (
	"strconv"
	"time"
)

func intPtr(v int) *int {
	return &v
}

func strPtr(v string) *string {
	return &v
}

func strconvItoa(v int) string {
	return strconv.Itoa(v)
}

func timeNow() time.Time {
	return time.Now()
}
