package common

import "strings"

type DatabaseType string

const (
	DatabaseTypeMySQL      DatabaseType = "mysql"
	DatabaseTypeSQLite     DatabaseType = "sqlite"
	DatabaseTypePostgreSQL DatabaseType = "postgres"
	DatabaseTypeClickHouse DatabaseType = "clickhouse"
)

var mainDatabaseType = DatabaseTypeSQLite
var logDatabaseType = DatabaseTypeSQLite

func MainDatabaseType() DatabaseType {
	return mainDatabaseType
}

func LogDatabaseType() DatabaseType {
	return logDatabaseType
}

func SetMainDatabaseType(databaseType DatabaseType) {
	mainDatabaseType = databaseType
}

func SetLogDatabaseType(databaseType DatabaseType) {
	logDatabaseType = databaseType
}

func SetDatabaseTypes(mainType DatabaseType, logType DatabaseType) {
	mainDatabaseType = mainType
	logDatabaseType = logType
}

func UsingMainDatabase(databaseType DatabaseType) bool {
	return mainDatabaseType == databaseType
}

func UsingLogDatabase(databaseType DatabaseType) bool {
	return logDatabaseType == databaseType
}

// Primary SQLite DSN pragmas. Keep conservative durability for the main DB.
const sqliteConnectionPragmas = "_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)"

// Message-log SQLite pragmas: still WAL for concurrency, but NORMAL sync and
// memory temp store cut fsync/write amplification on large BLOB inserts.
const msgLogSQLiteConnectionPragmas = "_pragma=busy_timeout(30000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=temp_store(MEMORY)"

// MsgLogSQLiteFileName is the on-disk basename (no query string) for message-log.db.
const MsgLogSQLiteFileName = "message-log.db"

func sqliteDSNWithPragmas(path, pragmas string) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + pragmas
}

func sqliteDSN(path string) string {
	return sqliteDSNWithPragmas(path, sqliteConnectionPragmas)
}

func msgLogSQLiteDSN(path string) string {
	return sqliteDSNWithPragmas(path, msgLogSQLiteConnectionPragmas)
}

var SQLitePath = sqliteDSN("one-api.db")
var MsgLogSQLitePath = msgLogSQLiteDSN(MsgLogSQLiteFileName)
