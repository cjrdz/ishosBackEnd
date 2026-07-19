// locks.go — PostgreSQL advisory-lock helpers for single-instance background jobs.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"math"
)

// AdvisoryLockKey derives a 64-bit advisory-lock key from an arbitrary string.
func AdvisoryLockKey(name string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(name))
	sum := h.Sum64()
	return int64(sum & math.MaxInt64)
}

// TryAdvisoryLock attempts to acquire a PostgreSQL session-level advisory lock
// without waiting. It returns a release function that must be called to free the
// lock and the underlying connection. If the lock is already held by another
// session, it returns (false, nil, nil).
func TryAdvisoryLock(ctx context.Context, rawDB *sql.DB, key int64) (bool, func(), error) {
	conn, err := rawDB.Conn(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("acquire db connection for advisory lock: %w", err)
	}

	release := func() {
		// Best-effort unlock and connection close.
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", key)
		_ = conn.Close()
	}

	var acquired bool
	if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&acquired); err != nil {
		_ = conn.Close()
		return false, nil, fmt.Errorf("pg_try_advisory_lock: %w", err)
	}
	if !acquired {
		_ = conn.Close()
		return false, nil, nil
	}

	return true, release, nil
}
