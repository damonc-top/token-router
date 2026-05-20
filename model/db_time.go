package model

import "github.com/QuantumNous/new-api/common"

// GetDBTimestamp returns a UNIX timestamp.
// On SQLite, uses application time directly to avoid connection pool contention.
// On MySQL/PostgreSQL, queries the DB to get server time.
func GetDBTimestamp() int64 {
	if common.UsingSQLite {
		return common.GetTimestamp()
	}
	var ts int64
	var err error
	if common.UsingPostgreSQL {
		err = DB.Raw("SELECT EXTRACT(EPOCH FROM NOW())::bigint").Scan(&ts).Error
	} else {
		err = DB.Raw("SELECT UNIX_TIMESTAMP()").Scan(&ts).Error
	}
	if err != nil || ts <= 0 {
		return common.GetTimestamp()
	}
	return ts
}
