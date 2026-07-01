package model

import "github.com/QuantumNous/new-api/common"

// GetDBTimestamp returns a UNIX timestamp.
// On SQLite, uses application time directly to avoid connection pool contention.
// On MySQL/PostgreSQL, queries the DB to get server time.
func GetDBTimestamp() int64 {
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		return common.GetTimestamp()
	}
	var ts int64
	var err error
	switch {
	case common.UsingMainDatabase(common.DatabaseTypePostgreSQL):
		err = DB.Raw("SELECT EXTRACT(EPOCH FROM NOW())::bigint").Scan(&ts).Error
	case common.UsingMainDatabase(common.DatabaseTypeSQLite):
		err = DB.Raw("SELECT strftime('%s','now')").Scan(&ts).Error
	default:
		err = DB.Raw("SELECT UNIX_TIMESTAMP()").Scan(&ts).Error
	}
	if err != nil || ts <= 0 {
		return common.GetTimestamp()
	}
	return ts
}
