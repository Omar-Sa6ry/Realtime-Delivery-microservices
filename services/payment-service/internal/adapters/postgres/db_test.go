package postgres

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// withSeams swaps the package-level dial/retry knobs for the duration of a test.
func withSeams(t *testing.T, open func(driverName, dsn string) (*sql.DB, error), attempts int, wait time.Duration) {
	t.Helper()
	origOpen, origAttempts, origWait := openConnection, pingAttempts, pingRetryWait
	openConnection = open
	pingAttempts = attempts
	pingRetryWait = wait
	t.Cleanup(func() {
		openConnection, pingAttempts, pingRetryWait = origOpen, origAttempts, origWait
	})
}

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db, mock
}

func TestEnsureDatabaseExistsOpenFailure(t *testing.T) {
	withSeams(t, func(string, string) (*sql.DB, error) {
		return nil, errors.New("malformed DSN")
	}, 1, 0)

	err := EnsureDatabaseExists("u", "p", "localhost", "5432", "payments", "disable")
	require.Error(t, err)
	assert.ErrorContains(t, err, "open default db")
}

func TestEnsureDatabaseExistsPingFailure(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectPing().WillReturnError(errors.New("connection refused"))
	withSeams(t, func(string, string) (*sql.DB, error) { return db, nil }, 1, time.Millisecond)

	err := EnsureDatabaseExists("u", "p", "localhost", "5432", "payments", "disable")
	require.Error(t, err)
	assert.ErrorContains(t, err, "ping default postgres db failed")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureDatabaseExistsWhenDatabaseAlreadyExists(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectPing()
	mock.ExpectQuery("SELECT EXISTS").WithArgs("payments").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	withSeams(t, func(string, string) (*sql.DB, error) { return db, nil }, 1, 0)

	require.NoError(t, EnsureDatabaseExists("u", "p", "localhost", "5432", "payments", "disable"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureDatabaseExistsCreatesMissingDatabase(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectPing()
	mock.ExpectQuery("SELECT EXISTS").WithArgs("brand_new_db").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(`CREATE DATABASE "brand_new_db"`).WillReturnResult(sqlmock.NewResult(0, 1))
	withSeams(t, func(string, string) (*sql.DB, error) { return db, nil }, 1, 0)

	require.NoError(t, EnsureDatabaseExists("u", "p", "localhost", "5432", "brand_new_db", "disable"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureDatabaseExistsCheckFailure(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectPing()
	mock.ExpectQuery("SELECT EXISTS").WillReturnError(errors.New("permission denied"))
	withSeams(t, func(string, string) (*sql.DB, error) { return db, nil }, 1, 0)

	err := EnsureDatabaseExists("u", "p", "localhost", "5432", "payments", "disable")
	require.Error(t, err)
	assert.ErrorContains(t, err, "check db exists")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureDatabaseExistsCreateFailure(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectPing()
	mock.ExpectQuery("SELECT EXISTS").WithArgs("payments").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	mock.ExpectExec(`CREATE DATABASE "payments"`).WillReturnError(errors.New("already exists"))
	withSeams(t, func(string, string) (*sql.DB, error) { return db, nil }, 1, 0)

	err := EnsureDatabaseExists("u", "p", "localhost", "5432", "payments", "disable")
	require.Error(t, err)
	assert.ErrorContains(t, err, "create database payments")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConnectEmptyDSN(t *testing.T) {
	db, err := Connect("", "u", "p", "localhost", "5432", "payments", "disable")
	require.Error(t, err)
	assert.Nil(t, db)
	assert.ErrorContains(t, err, "DSN is empty")
}

func TestConnectOpenFailure(t *testing.T) {
	withSeams(t, func(string, string) (*sql.DB, error) {
		return nil, errors.New("malformed DSN")
	}, 1, 0)

	db, err := Connect("postgres://x", "", "", "", "", "", "")
	require.Error(t, err)
	assert.Nil(t, db)
	assert.ErrorContains(t, err, "failed to open connection")
}

func TestConnectWarnsWhenEnsureDatabaseExistsFails(t *testing.T) {
	db, mock := newMockDB(t)
	calls := 0
	withSeams(t, func(string, string) (*sql.DB, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("bootstrap db unreachable")
		}
		return db, nil
	}, 1, 0)

	mock.ExpectPing()
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS payments").WillReturnResult(sqlmock.NewResult(0, 0))

	got, err := Connect("postgres://host/payments", "u", "p", "localhost", "5432", "payments", "disable")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 2, calls)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConnectPingFailure(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectPing().WillReturnError(errors.New("server not ready"))
	withSeams(t, func(string, string) (*sql.DB, error) { return db, nil }, 1, time.Millisecond)

	got, err := Connect("postgres://host/payments", "", "", "", "", "", "")
	require.Error(t, err)
	assert.Nil(t, got)
	assert.ErrorContains(t, err, "ping failed after retries")
}

func TestConnectSchemaFailure(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectPing()
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS payments").WillReturnError(errors.New("ddl rejected"))
	withSeams(t, func(string, string) (*sql.DB, error) { return db, nil }, 1, 0)

	got, err := Connect("postgres://host/payments", "", "", "", "", "", "")
	require.Error(t, err)
	assert.Nil(t, got)
	assert.ErrorContains(t, err, "failed to ensure schema")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConnectSuccess(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectPing()
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS payments").WillReturnResult(sqlmock.NewResult(0, 0))
	withSeams(t, func(string, string) (*sql.DB, error) { return db, nil }, 1, 0)

	got, err := Connect("postgres://host/payments", "", "", "", "", "payments", "")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, db, got)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConnectSkipsBootstrapWhenCredentialsMissing(t *testing.T) {
	db, mock := newMockDB(t)
	calls := 0
	withSeams(t, func(string, string) (*sql.DB, error) {
		calls++
		return db, nil
	}, 1, 0)

	mock.ExpectPing()
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS payments").WillReturnResult(sqlmock.NewResult(0, 0))

	_, err := Connect("postgres://host/payments", "", "", "", "5432", "", "")
	require.NoError(t, err)
	assert.Equal(t, 1, calls)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestEnsureSchemaSuccessAndFailure(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS payments").WillReturnResult(sqlmock.NewResult(0, 0))
	require.NoError(t, EnsureSchema(t.Context(), db))
	assert.NoError(t, mock.ExpectationsWereMet())

	db2, mock2 := newMockDB(t)
	mock2.ExpectExec("CREATE TABLE IF NOT EXISTS payments").WillReturnError(errors.New("boom"))
	err := EnsureSchema(t.Context(), db2)
	require.Error(t, err)
	assert.ErrorContains(t, err, "EnsureSchema")
	assert.NoError(t, mock2.ExpectationsWereMet())
}

func TestClose(t *testing.T) {
	Close(nil)

	db, _ := newMockDB(t)
	Close(db)
}
