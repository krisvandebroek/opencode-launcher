package opencodestorage

// normalizeUnixMillisFromSQLite normalizes SQLite time fields that may be
// stored in seconds or milliseconds.
func normalizeUnixMillisFromSQLite(ts int64) int64 {
	// If it's already in ms (e.g. 13-digit unix millis), keep it.
	// 99_999_999_999 is ~5138-11-16 in seconds; anything larger is almost
	// certainly milliseconds.
	if ts > 99_999_999_999 {
		return ts
	}
	if ts <= 0 {
		return ts
	}
	return ts * 1000
}
