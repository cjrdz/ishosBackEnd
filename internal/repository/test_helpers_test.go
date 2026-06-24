// test_helpers_test.go — Shared test helpers for repository integration tests.
package repository

import (
	"strconv"
	"time"
)

func strconvItoa(i int) string {
	return strconv.Itoa(i)
}

func timeNow() time.Time {
	return time.Now()
}

func strPtr(s string) *string {
	return &s
}
