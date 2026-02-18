package opencodestorage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// CheckStorageReadable validates the configured data sources.
//
// Default behavior (useLegacy=false): require SQLite to be readable.
// Legacy behavior (useLegacy=true): accept JSON and/or SQLite.
func CheckStorageReadable(storageRoot, dbPath string, useLegacy bool, disableSQLite bool) error {
	if !useLegacy {
		if disableSQLite {
			return errors.New("sqlite disabled and legacy disabled")
		}
		return checkSQLiteReadable(dbPath)
	}

	jsonErr := checkJSONReadable(storageRoot)
	if disableSQLite {
		return jsonErr
	}
	sqliteErr := checkSQLiteReadable(dbPath)
	if jsonErr == nil || sqliteErr == nil {
		return nil
	}
	return fmt.Errorf("json unreadable: %w; sqlite unreadable: %v", jsonErr, sqliteErr)
}

func checkJSONReadable(storageRoot string) error {
	if storageRoot == "" {
		return errors.New("empty storage root")
	}
	st, err := os.Stat(storageRoot)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("not a directory: %s", storageRoot)
	}
	// Ensure we can read the project dir; this is the minimum for oc to function.
	_, err = os.ReadDir(filepath.Join(storageRoot, "storage", "project"))
	return err
}

func checkSQLiteReadable(dbPath string) error {
	st, err := OpenSQLiteStore(dbPath)
	if err != nil {
		return err
	}
	return st.Close()
}
