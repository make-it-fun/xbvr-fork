package models

import (
	"strings"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	_ "github.com/mattn/go-sqlite3"
	"github.com/thoas/go-funk"
	"github.com/xbapps/xbvr/pkg/common"
	"github.com/xo/dburl"
)

var log = &common.Log
var dbConn *dburl.URL
var supportedDB = []string{"mysql", "sqlite3"}
var commonConnection *gorm.DB

func parseDBConnString() {
	var err error
	dbConn, err = dburl.Parse(common.DATABASE_URL)
	if err != nil {
		log.Fatal("Error parsing database connection ", common.DATABASE_URL, err)
	}
	_, ok := gorm.GetDialect(dbConn.Driver)
	if !ok || !funk.Contains(supportedDB, dbConn.Driver) {
		log.Fatal("Unsupported database: ", dbConn.Short())
	}
}

func GetDBConn() *dburl.URL {
	return dbConn
}

// isTransient reports whether err is a temporary SQLite condition that's worth
// retrying (lock/busy contention). Permanent errors — UNIQUE/constraint
// violations, schema errors, etc. — return false so the retry bails out
// immediately instead of hammering the same doomed write 10 times.
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database is busy") ||
		strings.Contains(msg, "table is locked") ||
		strings.Contains(msg, "sqlite_busy")
}

// isDuplicateName reports whether err is a unique-constraint violation on
// actors.name — the signature of two concurrent scrape goroutines racing to
// create the same new actor. The loser can recover by adopting the winning row
// (see Actor.Save) instead of failing.
func isDuplicateName(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "actors.name") &&
		(strings.Contains(msg, "unique constraint failed") || strings.Contains(msg, "duplicate"))
}

// SaveWithRetry persists i, retrying only on transient SQLite contention. A
// permanent failure (or exhausted retries) is logged at error level and the
// error is returned to the caller. It must NEVER call log.Fatal: a single
// failed row-save should skip that row, not take down the whole server.
func SaveWithRetry(db *gorm.DB, i interface{}) error {
	err := retry.Do(
		func() error {
			return db.Save(i).Error
		},
		retry.RetryIf(isTransient),
	)

	if err != nil {
		log.Error("Failed to save: ", err)
	}

	return err
}

func GetDB() (*gorm.DB, error) {
	if common.EnvConfig.DebugSQL {
		log.Debug("Getting DB handle from ", common.GetCallerFunctionName())
	}

	var db *gorm.DB
	var err error

	err = retry.Do(
		func() error {
			db, err = gorm.Open(dbConn.Driver, dbConn.DSN)
			db.LogMode(common.EnvConfig.DebugSQL)
			if err != nil {
				return err
			}
			return nil
		},
	)

	if err != nil {
		log.Fatal("Failed to connect to database ", err)
	}

	return db, nil
}

func GetCommonDB() (*gorm.DB, error) {
	if common.EnvConfig.DebugSQL {
		log.Debug("Getting Common DB handle from ", common.GetCallerFunctionName())
	}

	var err error

	if commonConnection != nil {
		return commonConnection, nil
	}
	err = retry.Do(
		func() error {
			commonConnection, err = gorm.Open(dbConn.Driver, dbConn.DSN)
			commonConnection.LogMode(common.EnvConfig.DebugSQL)
			commonConnection.DB().SetConnMaxIdleTime(4 * time.Minute)
			if common.DBConnectionPoolSize > 0 {
				commonConnection.DB().SetMaxOpenConns(common.DBConnectionPoolSize)
			}
			if err != nil {
				return err
			}
			return nil
		},
	)

	if err != nil {
		log.Fatal("Failed to connect to database ", err)
	}

	return commonConnection, nil
}

// Lock functions

func CreateLock(lock string) {
	obj := KV{Key: "lock-" + lock, Value: "1"}
	obj.Save()

	common.PublishWS("lock.change", map[string]interface{}{"name": lock, "locked": true})
}

func CheckLock(lock string) bool {
	db, _ := GetDB()
	defer db.Close()

	var obj KV
	err := db.Where(&KV{Key: "lock-" + lock}).First(&obj).Error

	return err == nil
}

func RemoveLock(lock string) {
	db, _ := GetDB()
	defer db.Close()

	var obj KV
	db.Where(&KV{Key: "lock-" + lock}).Delete(&obj)

	common.PublishWS("lock.change", map[string]interface{}{"name": lock, "locked": false})
}

func RemoveAllLocks() {
	db, _ := GetDB()
	defer db.Close()

	var locks []KV
	err := db.Where("`key` like 'lock-%'").Find(&locks).Error
	if err != nil {
		return
	}

	for _, lock := range locks {
		lockName := strings.Replace(lock.Key, "lock-", "", 1)
		RemoveLock(lockName)
	}
}

func init() {
	common.InitPaths()
	common.InitLogging()
	parseDBConnString()
	GetCommonDB()
}
