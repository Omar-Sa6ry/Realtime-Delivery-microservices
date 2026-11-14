package postgres

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/payment-service/internal/domain"
)

// ---------------------------------------------------------------------------
// fake driver whose cursor fails, so rows.Err() branches become reachable
// ---------------------------------------------------------------------------

type failingRowsDriver struct{}

func (failingRowsDriver) Open(string) (driver.Conn, error) { return failingConn{}, nil }

type failingConn struct{}

func (failingConn) Prepare(string) (driver.Stmt, error) { return failingStmt{}, nil }
func (failingConn) Close() error                        { return nil }
func (failingConn) Begin() (driver.Tx, error)           { return nil, errors.New("no transactions") }

type failingStmt struct{}

func (failingStmt) Close() error                                    { return nil }
func (failingStmt) NumInput() int                                   { return -1 }
func (failingStmt) Exec([]driver.Value) (driver.Result, error)      { return nil, errors.New("not supported") }
func (failingStmt) Query([]driver.Value) (driver.Rows, error)       { return &cursorFailRows{}, nil }

type cursorFailRows struct{}

func (*cursorFailRows) Columns() []string { return []string{"id"} }
func (*cursorFailRows) Close() error      { return nil }
func (*cursorFailRows) Next([]driver.Value) error {
	return errors.New("cursor read failure")
}

func init() {
	sql.Register("payment-service-failing-rows", failingRowsDriver{})
}

func newFailingRowsDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("payment-service-failing-rows", "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// ---------------------------------------------------------------------------
// row helpers
// ---------------------------------------------------------------------------

func paymentColumns() []string {
	return []string{
		"id", "delivery_id", "user_id", "amount_minor", "currency", "status",
		"provider", "provider_payment_id", "gateway_session_id",
		"authorized_amount_minor", "captured_amount_minor", "refunded_amount_minor", "pending_refund_minor",
		"version", "correlation_id", "causation_id",
		"created_at", "updated_at", "authorized_at", "captured_at", "cancelled_at", "failed_at",
	}
}

func addPaymentRow(rows *sqlmock.Rows, id, status string, version int64) *sqlmock.Rows {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ts := now
	return rows.AddRow(
		id, "del_1", "usr_1", int64(5000), "USD", status,
		"stripe", "pi_1", "cs_1",
		int64(0), int64(0), int64(0), int64(0),
		version, "corr_1", "cause_1",
		now, now, &ts, nil, nil, nil,
	)
}

func attemptColumns() []string {
	return []string{
		"id", "payment_id", "attempt_number", "operation", "status", "provider",
		"provider_idempotency_key", "provider_transaction_id", "error_code", "error_category",
		"safe_error_message", "created_at", "started_at", "completed_at",
	}
}

func addAttemptRow(rows *sqlmock.Rows) *sqlmock.Rows {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ts := now
	return rows.AddRow(
		"att_1", "pay_1", 1, "AUTHORIZE", "SUCCEEDED", "stripe",
		"idem_1", "pi_1", "", "",
		"", now, &ts, nil,
	)
}

func stuckColumns() []string {
	return []string{
		"id", "payment_id", "operation", "provider", "provider_idempotency_key", "provider_transaction_id",
	}
}

func refundColumns() []string {
	return []string{
		"id", "payment_id", "delivery_id", "amount_minor", "currency", "status", "reason",
		"provider_refund_id", "idempotency_key", "created_at", "updated_at", "completed_at", "failed_at", "failure_reason",
	}
}

func addRefundRow(rows *sqlmock.Rows) *sqlmock.Rows {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ts := now
	return rows.AddRow(
		"ref_1", "pay_1", "del_1", int64(500), "USD", "REFUND_SUCCEEDED", "duplicate",
		"re_1", "idem_1", now, now, &ts, nil, "",
	)
}

// ---------------------------------------------------------------------------
// payment repository
// ---------------------------------------------------------------------------

func TestPaymentCreate(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewPaymentRepository(db)
	p := &domain.Payment{ID: "pay_1"}

	mock.ExpectExec(`INSERT INTO payments \(`).WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, repo.Create(t.Context(), p))

	mock.ExpectExec(`INSERT INTO payments \(`).WillReturnError(errors.New("violates constraint"))
	require.Error(t, repo.Create(t.Context(), p))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentFindByID(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewPaymentRepository(db)

	mock.ExpectQuery(`WHERE id = \$1`).WithArgs("pay_1").
		WillReturnRows(addPaymentRow(sqlmock.NewRows(paymentColumns()), "pay_1", "AUTHORIZED", 4))
	p, err := repo.FindByID(t.Context(), "pay_1")
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.Equal(t, int64(4), p.Version)
	assert.Equal(t, domain.PaymentStatus("AUTHORIZED"), p.Status)

	mock.ExpectQuery(`WHERE id = \$1`).WithArgs("pay_2").WillReturnRows(sqlmock.NewRows(paymentColumns()))
	p, err = repo.FindByID(t.Context(), "pay_2")
	assert.Nil(t, p)
	assert.ErrorIs(t, err, domain.ErrPaymentNotFound)

	mock.ExpectQuery(`WHERE id = \$1`).WithArgs("pay_3").WillReturnError(errors.New("connection lost"))
	p, err = repo.FindByID(t.Context(), "pay_3")
	assert.Nil(t, p)
	require.Error(t, err)
	assert.ErrorContains(t, err, "payment_repo.FindByID")

	mock.ExpectQuery(`WHERE id = \$1`).WithArgs("pay_4").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("pay_4"))
	p, err = repo.FindByID(t.Context(), "pay_4")
	assert.Nil(t, p)
	require.Error(t, err)
	assert.ErrorContains(t, err, "payment_repo.FindByID")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentFindByDeliveryID(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewPaymentRepository(db)

	mock.ExpectQuery(`WHERE delivery_id = \$1`).WithArgs("del_1").
		WillReturnRows(addPaymentRow(sqlmock.NewRows(paymentColumns()), "pay_1", "PENDING", 1))
	p, err := repo.FindByDeliveryID(t.Context(), "del_1")
	require.NoError(t, err)
	require.NotNil(t, p)

	mock.ExpectQuery(`WHERE delivery_id = \$1`).WithArgs("del_2").WillReturnRows(sqlmock.NewRows(paymentColumns()))
	p, err = repo.FindByDeliveryID(t.Context(), "del_2")
	assert.Nil(t, p)
	assert.ErrorIs(t, err, domain.ErrPaymentNotFound)

	mock.ExpectQuery(`WHERE delivery_id = \$1`).WithArgs("del_3").WillReturnError(errors.New("boom"))
	p, err = repo.FindByDeliveryID(t.Context(), "del_3")
	assert.Nil(t, p)
	require.Error(t, err)
	assert.ErrorContains(t, err, "payment_repo.FindByDeliveryID")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentFindByProviderPaymentID(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewPaymentRepository(db)

	mock.ExpectQuery(`WHERE provider_payment_id = \$1`).WithArgs("pi_1").
		WillReturnRows(addPaymentRow(sqlmock.NewRows(paymentColumns()), "pay_1", "CAPTURED", 6))
	p, err := repo.FindByProviderPaymentID(t.Context(), "pi_1")
	require.NoError(t, err)
	require.NotNil(t, p)

	mock.ExpectQuery(`WHERE provider_payment_id = \$1`).WithArgs("pi_missing").WillReturnRows(sqlmock.NewRows(paymentColumns()))
	p, err = repo.FindByProviderPaymentID(t.Context(), "pi_missing")
	assert.Nil(t, p)
	assert.ErrorIs(t, err, domain.ErrPaymentNotFound)

	mock.ExpectQuery(`WHERE provider_payment_id = \$1`).WithArgs("pi_err").WillReturnError(errors.New("boom"))
	p, err = repo.FindByProviderPaymentID(t.Context(), "pi_err")
	assert.Nil(t, p)
	require.Error(t, err)
	assert.ErrorContains(t, err, "payment_repo.FindByProviderPaymentID")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentFindByUserID(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewPaymentRepository(db)

	rows := addPaymentRow(sqlmock.NewRows(paymentColumns()), "pay_1", "PENDING", 1)
	rows = addPaymentRow(rows, "pay_2", "PENDING", 1)
	mock.ExpectQuery(`LIMIT 100`).WithArgs("usr_1").WillReturnRows(rows)
	list, err := repo.FindByUserID(t.Context(), "usr_1")
	require.NoError(t, err)
	assert.Len(t, list, 2)

	mock.ExpectQuery(`LIMIT 100`).WithArgs("usr_2").WillReturnError(errors.New("query failed"))
	list, err = repo.FindByUserID(t.Context(), "usr_2")
	assert.Nil(t, list)
	require.Error(t, err)
	assert.ErrorContains(t, err, "payment_repo.FindByUserID")

	mock.ExpectQuery(`LIMIT 100`).WithArgs("usr_3").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("pay_3"))
	list, err = repo.FindByUserID(t.Context(), "usr_3")
	assert.Nil(t, list)
	require.Error(t, err)
	assert.ErrorContains(t, err, "scan payment")

	mock.ExpectQuery(`LIMIT 100`).WithArgs("usr_4").WillReturnRows(sqlmock.NewRows(paymentColumns()))
	list, err = repo.FindByUserID(t.Context(), "usr_4")
	require.NoError(t, err)
	assert.Empty(t, list)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentListDefaultsAndPagination(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewPaymentRepository(db)

	// page/limit out of range fall back to 1 / 20
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM payments$`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectQuery(`LIMIT \$1 OFFSET \$2`).
		WillReturnRows(sqlmock.NewRows(paymentColumns()))
	list, total, err := repo.List(t.Context(), 0, 0, "")
	require.NoError(t, err)
	assert.Empty(t, list)
	assert.Equal(t, 0, total)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM payments$`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(0)))
	mock.ExpectQuery(`LIMIT \$1 OFFSET \$2`).
		WillReturnRows(sqlmock.NewRows(paymentColumns()))
	_, _, err = repo.List(t.Context(), 1, 500, "")
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentListWithUserID(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewPaymentRepository(db)

	rows := addPaymentRow(sqlmock.NewRows(paymentColumns()), "pay_1", "PENDING", 1)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM payments WHERE user_id`).
		WithArgs("usr_1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`LIMIT \$2 OFFSET \$3`).
		WithArgs("usr_1", 10, 10).
		WillReturnRows(rows)
	list, total, err := repo.List(t.Context(), 2, 10, "usr_1")
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, 1, total)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentListCountErrors(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewPaymentRepository(db)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM payments$`).WillReturnError(errors.New("count failed"))
	_, _, err := repo.List(t.Context(), 1, 10, "")
	require.Error(t, err)
	assert.ErrorContains(t, err, "payment_repo.List count")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM payments WHERE user_id`).WillReturnError(errors.New("count failed"))
	_, _, err = repo.List(t.Context(), 1, 10, "usr_1")
	require.Error(t, err)
	assert.ErrorContains(t, err, "payment_repo.List count")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentListQueryAndScanErrors(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewPaymentRepository(db)

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM payments$`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`LIMIT \$1 OFFSET \$2`).WillReturnError(errors.New("query failed"))
	_, _, err := repo.List(t.Context(), 1, 10, "")
	require.Error(t, err)
	assert.ErrorContains(t, err, "payment_repo.List")

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM payments$`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(1)))
	mock.ExpectQuery(`LIMIT \$1 OFFSET \$2`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("pay_1"))
	_, _, err = repo.List(t.Context(), 1, 10, "")
	require.Error(t, err)
	assert.ErrorContains(t, err, "scan payment")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentFindByIDs(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewPaymentRepository(db)

	list, err := repo.FindByIDs(t.Context(), nil)
	require.NoError(t, err)
	assert.Nil(t, list)

	rows := addPaymentRow(sqlmock.NewRows(paymentColumns()), "pay_1", "PENDING", 1)
	mock.ExpectQuery(`ANY\(\$1\)`).WillReturnRows(rows)
	list, err = repo.FindByIDs(t.Context(), []string{"pay_1"})
	require.NoError(t, err)
	assert.Len(t, list, 1)

	mock.ExpectQuery(`ANY\(\$1\)`).WillReturnError(errors.New("bad array"))
	list, err = repo.FindByIDs(t.Context(), []string{"pay_1"})
	assert.Nil(t, list)
	require.Error(t, err)
	assert.ErrorContains(t, err, "payment_repo.FindByIDs")

	mock.ExpectQuery(`ANY\(\$1\)`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("pay_1"))
	list, err = repo.FindByIDs(t.Context(), []string{"pay_1"})
	assert.Nil(t, list)
	require.Error(t, err)
	assert.ErrorContains(t, err, "scan payment")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPaymentFindByIDsRowsError(t *testing.T) {
	repo := NewPaymentRepository(newFailingRowsDB(t))
	list, err := repo.FindByIDs(t.Context(), []string{"pay_1"})
	require.Error(t, err)
	assert.Nil(t, list)
	assert.ErrorContains(t, err, "rows error")
}

func TestPaymentUpdateConditional(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewPaymentRepository(db)
	p := &domain.Payment{ID: "pay_1", Version: 5, Status: domain.PaymentStatusPending}

	mock.ExpectExec(`version=\$15`).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.UpdateConditional(t.Context(), p, 4))

	mock.ExpectExec(`version=\$15`).WillReturnError(errors.New("write failed"))
	err := repo.UpdateConditional(t.Context(), p, 4)
	require.Error(t, err)
	assert.ErrorContains(t, err, "payment_repo.UpdateConditional")

	mock.ExpectExec(`version=\$15`).WillReturnResult(sqlmock.NewResult(0, 0))
	err = repo.UpdateConditional(t.Context(), p, 4)
	assert.ErrorIs(t, err, domain.ErrConcurrentModification)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// attempt repository
// ---------------------------------------------------------------------------

func TestAttemptCreateAndUpdateStatus(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewAttemptRepository(db)
	a := &domain.Attempt{ID: "att_1", PaymentID: "pay_1", AttemptNumber: 1, Operation: "AUTHORIZE"}

	mock.ExpectExec(`INSERT INTO payment_attempts`).WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, repo.Create(t.Context(), a))

	mock.ExpectExec(`INSERT INTO payment_attempts`).WillReturnError(errors.New("boom"))
	require.Error(t, repo.Create(t.Context(), a))

	mock.ExpectExec(`UPDATE payment_attempts SET`).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.UpdateStatus(t.Context(), "att_1", domain.OperationStatus("SUCCEEDED"), "pi_1"))

	mock.ExpectExec(`UPDATE payment_attempts SET`).WillReturnError(errors.New("boom"))
	require.Error(t, repo.UpdateStatus(t.Context(), "att_1", domain.OperationStatus("FAILED"), ""))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAttemptFindByPaymentID(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewAttemptRepository(db)

	mock.ExpectQuery(`FROM payment_attempts WHERE payment_id`).WithArgs("pay_1").
		WillReturnRows(addAttemptRow(sqlmock.NewRows(attemptColumns())))
	attempts, err := repo.FindByPaymentID(t.Context(), "pay_1")
	require.NoError(t, err)
	require.Len(t, attempts, 1)
	assert.Equal(t, domain.OperationStatus("SUCCEEDED"), attempts[0].Status)

	mock.ExpectQuery(`FROM payment_attempts WHERE payment_id`).WillReturnError(errors.New("boom"))
	_, err = repo.FindByPaymentID(t.Context(), "pay_1")
	require.Error(t, err)
	assert.ErrorContains(t, err, "attempt_repo.FindByPaymentID")

	mock.ExpectQuery(`FROM payment_attempts WHERE payment_id`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("att_1"))
	_, err = repo.FindByPaymentID(t.Context(), "pay_1")
	require.Error(t, err)
	assert.ErrorContains(t, err, "attempt_repo: scan")

	mock.ExpectQuery(`FROM payment_attempts WHERE payment_id`).
		WillReturnRows(sqlmock.NewRows(attemptColumns()))
	attempts, err = repo.FindByPaymentID(t.Context(), "pay_1")
	require.NoError(t, err)
	assert.Empty(t, attempts)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAttemptFindStuckAndUnknown(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewAttemptRepository(db)

	rows := sqlmock.NewRows(stuckColumns()).AddRow("att_1", "pay_1", "AUTHORIZE", "stripe", "idem_1", "pi_1")
	mock.ExpectQuery(`WHERE status = 'PROCESSING'`).WillReturnRows(rows)
	stuck, err := repo.FindStuck(t.Context(), 60)
	require.NoError(t, err)
	require.Len(t, stuck, 1)
	assert.Equal(t, domain.OperationStatusProcessing, stuck[0].Status)

	mock.ExpectQuery(`WHERE status = 'PROCESSING'`).WillReturnError(errors.New("boom"))
	_, err = repo.FindStuck(t.Context(), 60)
	require.Error(t, err)
	assert.ErrorContains(t, err, "attempt_repo.FindStuck")

	mock.ExpectQuery(`WHERE status = 'PROCESSING'`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("att_1"))
	_, err = repo.FindStuck(t.Context(), 60)
	require.Error(t, err)
	assert.Error(t, err)

	mock.ExpectQuery(`WHERE status = 'UNKNOWN'`).WillReturnRows(
		sqlmock.NewRows(stuckColumns()).AddRow("att_2", "pay_2", "CAPTURE", "stripe", "idem_2", "pi_2"),
	)
	unknown, err := repo.FindUnknown(t.Context(), 20)
	require.NoError(t, err)
	require.Len(t, unknown, 1)
	assert.Equal(t, domain.OperationStatusUnknown, unknown[0].Status)

	mock.ExpectQuery(`WHERE status = 'UNKNOWN'`).WillReturnError(errors.New("boom"))
	_, err = repo.FindUnknown(t.Context(), 20)
	require.Error(t, err)
	assert.ErrorContains(t, err, "attempt_repo.FindUnknown")

	mock.ExpectQuery(`WHERE status = 'UNKNOWN'`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("att_2"))
	_, err = repo.FindUnknown(t.Context(), 20)
	require.Error(t, err)
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// refund repository
// ---------------------------------------------------------------------------

func TestRefundCreateAndUpdateStatus(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRefundRepository(db)
	r := &domain.Refund{ID: "ref_1", PaymentID: "pay_1"}

	mock.ExpectExec(`INSERT INTO refunds`).WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, repo.Create(t.Context(), r))

	mock.ExpectExec(`INSERT INTO refunds`).WillReturnError(errors.New("boom"))
	require.Error(t, repo.Create(t.Context(), r))

	mock.ExpectExec(`UPDATE refunds SET`).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.UpdateStatus(t.Context(), "ref_1", domain.RefundStatus("REFUND_SUCCEEDED"), "re_1"))

	mock.ExpectExec(`UPDATE refunds SET`).WillReturnError(errors.New("boom"))
	require.Error(t, repo.UpdateStatus(t.Context(), "ref_1", domain.RefundStatus("REFUND_FAILED"), ""))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefundFindByID(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRefundRepository(db)

	mock.ExpectQuery(`FROM refunds WHERE id`).WithArgs("ref_1").
		WillReturnRows(addRefundRow(sqlmock.NewRows(refundColumns())))
	r, err := repo.FindByID(t.Context(), "ref_1")
	require.NoError(t, err)
	require.NotNil(t, r)
	assert.Equal(t, domain.RefundStatus("REFUND_SUCCEEDED"), r.Status)

	mock.ExpectQuery(`FROM refunds WHERE id`).WithArgs("ref_2").WillReturnRows(sqlmock.NewRows(refundColumns()))
	r, err = repo.FindByID(t.Context(), "ref_2")
	assert.Nil(t, r)
	assert.ErrorIs(t, err, domain.ErrRefundNotFound)

	mock.ExpectQuery(`FROM refunds WHERE id`).WithArgs("ref_3").WillReturnError(errors.New("boom"))
	r, err = repo.FindByID(t.Context(), "ref_3")
	assert.Nil(t, r)
	require.Error(t, err)
	assert.ErrorContains(t, err, "refund_repo.FindByID")

	mock.ExpectQuery(`FROM refunds WHERE id`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ref_4"))
	r, err = repo.FindByID(t.Context(), "ref_4")
	assert.Nil(t, r)
	require.Error(t, err)
	assert.ErrorContains(t, err, "refund_repo.FindByID")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestRefundFindByPaymentID(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRefundRepository(db)

	mock.ExpectQuery(`WHERE payment_id = \$1 ORDER BY`).WithArgs("pay_1").
		WillReturnRows(addRefundRow(sqlmock.NewRows(refundColumns())))
	refunds, err := repo.FindByPaymentID(t.Context(), "pay_1")
	require.NoError(t, err)
	assert.Len(t, refunds, 1)

	mock.ExpectQuery(`WHERE payment_id = \$1 ORDER BY`).WillReturnError(errors.New("boom"))
	_, err = repo.FindByPaymentID(t.Context(), "pay_1")
	require.Error(t, err)
	assert.ErrorContains(t, err, "refund_repo.FindByPaymentID")

	mock.ExpectQuery(`WHERE payment_id = \$1 ORDER BY`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("ref_1"))
	_, err = repo.FindByPaymentID(t.Context(), "pay_1")
	require.Error(t, err)
	assert.ErrorContains(t, err, "refund_repo: scan")

	mock.ExpectQuery(`WHERE payment_id = \$1 ORDER BY`).
		WillReturnRows(sqlmock.NewRows(refundColumns()))
	refunds, err = repo.FindByPaymentID(t.Context(), "pay_1")
	require.NoError(t, err)
	assert.Empty(t, refunds)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// outbox repository
// ---------------------------------------------------------------------------

func TestOutboxInsertAndProcessedEvent(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewOutboxRepository(db)

	mock.ExpectExec(`INSERT INTO payment_events_outbox`).WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, repo.Insert(t.Context(), "payment.completed", []byte("{}")))

	mock.ExpectExec(`INSERT INTO payment_events_outbox`).WillReturnError(errors.New("boom"))
	require.Error(t, repo.Insert(t.Context(), "payment.completed", []byte("{}")))

	mock.ExpectExec(`INSERT INTO processed_provider_events`).WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, repo.InsertProcessedProviderEvent(t.Context(), "stripe", "evt_1", "charge.refunded"))

	mock.ExpectExec(`INSERT INTO processed_provider_events`).WillReturnResult(sqlmock.NewResult(0, 0))
	err := repo.InsertProcessedProviderEvent(t.Context(), "stripe", "evt_1", "charge.refunded")
	assert.ErrorIs(t, err, domain.ErrDuplicateIdempotency)

	mock.ExpectExec(`INSERT INTO processed_provider_events`).WillReturnError(errors.New("boom"))
	require.Error(t, repo.InsertProcessedProviderEvent(t.Context(), "stripe", "evt_2", "charge.refunded"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOutboxFetchUnpublished(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewOutboxRepository(db)

	rows := sqlmock.NewRows([]string{"id", "event_type", "payload", "created_at"}).
		AddRow("out_1", "payment.completed", []byte(`{"a":1}`), int64(1700000000))
	mock.ExpectQuery(`FOR UPDATE SKIP LOCKED`).WithArgs(10).WillReturnRows(rows)
	events, err := repo.FetchUnpublished(t.Context(), 10)
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "out_1", events[0].ID)

	mock.ExpectQuery(`FOR UPDATE SKIP LOCKED`).WillReturnError(sql.ErrNoRows)
	events, err = repo.FetchUnpublished(t.Context(), 10)
	require.NoError(t, err)
	assert.Nil(t, events)

	mock.ExpectQuery(`FOR UPDATE SKIP LOCKED`).WillReturnError(errors.New("boom"))
	_, err = repo.FetchUnpublished(t.Context(), 10)
	require.Error(t, err)
	assert.ErrorContains(t, err, "outbox_repo.FetchUnpublished")

	mock.ExpectQuery(`FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow("out_1"))
	_, err = repo.FetchUnpublished(t.Context(), 10)
	require.Error(t, err)
	assert.ErrorContains(t, err, "outbox_repo: scan")

	mock.ExpectQuery(`FOR UPDATE SKIP LOCKED`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "event_type", "payload", "created_at"}))
	events, err = repo.FetchUnpublished(t.Context(), 10)
	require.NoError(t, err)
	assert.Empty(t, events)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestOutboxFetchRowsError(t *testing.T) {
	repo := NewOutboxRepository(newFailingRowsDB(t))
	events, err := repo.FetchUnpublished(t.Context(), 10)
	require.Error(t, err)
	assert.Nil(t, events)
}

func TestOutboxMarkAndCleanup(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewOutboxRepository(db)

	mock.ExpectExec(`SET published_at = NOW\(\)`).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.MarkPublished(t.Context(), "out_1"))

	mock.ExpectExec(`SET published_at = NOW\(\)`).WillReturnError(errors.New("boom"))
	require.Error(t, repo.MarkPublished(t.Context(), "out_1"))

	mock.ExpectExec(`SET failed_reason`).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.MarkFailed(t.Context(), "out_1", "timeout"))

	mock.ExpectExec(`SET failed_reason`).WillReturnError(errors.New("boom"))
	require.Error(t, repo.MarkFailed(t.Context(), "out_1", "timeout"))

	mock.ExpectExec(`DELETE FROM payment_events_outbox`).WillReturnResult(sqlmock.NewResult(0, 3))
	n, err := repo.Cleanup(t.Context(), 7)
	require.NoError(t, err)
	assert.Equal(t, int64(3), n)

	mock.ExpectExec(`DELETE FROM payment_events_outbox`).WillReturnError(errors.New("boom"))
	_, err = repo.Cleanup(t.Context(), 7)
	require.Error(t, err)
	assert.ErrorContains(t, err, "outbox_repo.Cleanup")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// idempotency + audit repositories
// ---------------------------------------------------------------------------

func TestIdempotencyStoreRetrieveDeleteExists(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewIdempotencyRepository(db)

	mock.ExpectExec(`INSERT INTO idempotency_keys`).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Store("k1", "payload"))

	mock.ExpectExec(`INSERT INTO idempotency_keys`).WillReturnError(errors.New("boom"))
	require.Error(t, repo.Store("k1", "payload"))

	mock.ExpectQuery(`SELECT result FROM idempotency_keys`).WithArgs("k1").
		WillReturnRows(sqlmock.NewRows([]string{"result"}).AddRow("payload"))
	got, ok := repo.Retrieve("k1")
	assert.True(t, ok)
	assert.Equal(t, "payload", got)

	mock.ExpectQuery(`SELECT result FROM idempotency_keys`).WithArgs("k2").
		WillReturnRows(sqlmock.NewRows([]string{"result"}))
	got, ok = repo.Retrieve("k2")
	assert.False(t, ok)
	assert.Empty(t, got)

	mock.ExpectQuery(`SELECT result FROM idempotency_keys`).WithArgs("k3").
		WillReturnError(errors.New("boom"))
	got, ok = repo.Retrieve("k3")
	assert.False(t, ok)
	assert.Empty(t, got)

	mock.ExpectExec(`DELETE FROM idempotency_keys`).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.Delete("k1"))

	mock.ExpectExec(`DELETE FROM idempotency_keys`).WillReturnResult(sqlmock.NewResult(0, 0))
	err := repo.Delete("k1")
	require.Error(t, err)
	assert.ErrorContains(t, err, "idempotency key not found")

	mock.ExpectExec(`DELETE FROM idempotency_keys`).WillReturnError(errors.New("boom"))
	require.Error(t, repo.Delete("k1"))

	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM idempotency_keys`).WithArgs("k1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	exists, err := repo.Exists("k1")
	require.NoError(t, err)
	assert.True(t, exists)

	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM idempotency_keys`).WithArgs("k1").
		WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))
	exists, err = repo.Exists("k1")
	require.NoError(t, err)
	assert.False(t, exists)

	mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM idempotency_keys`).WillReturnError(errors.New("boom"))
	exists, err = repo.Exists("k1")
	require.Error(t, err)
	assert.False(t, exists)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditLogEvent(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewAuditRepository(db)

	mock.ExpectExec(`INSERT INTO payment_audit`).WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, repo.LogEvent("payment.authorized", "pay_1", map[string]interface{}{"amount": 100}))

	// a channel value cannot be marshalled, so the repo falls back to "{}"
	mock.ExpectExec(`INSERT INTO payment_audit`).
		WithArgs("payment.failed", "pay_1", []byte("{}")).
		WillReturnResult(sqlmock.NewResult(1, 1))
	require.NoError(t, repo.LogEvent("payment.failed", "pay_1", map[string]interface{}{"ch": make(chan int)}))

	mock.ExpectExec(`INSERT INTO payment_audit`).WillReturnError(errors.New("boom"))
	require.Error(t, repo.LogEvent("payment.failed", "pay_1", nil))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestAuditGetEventsByPaymentID(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewAuditRepository(db)

	rows := sqlmock.NewRows([]string{"event_type", "details", "created_at"}).
		AddRow("payment.authorized", `{"amount":100}`, "2026-01-02T03:04:05Z")
	mock.ExpectQuery(`FROM payment_audit WHERE payment_id`).WithArgs("pay_1").WillReturnRows(rows)
	events, err := repo.GetEventsByPaymentID("pay_1")
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, "payment.authorized", events[0]["event_type"])

	mock.ExpectQuery(`FROM payment_audit WHERE payment_id`).WillReturnError(errors.New("boom"))
	_, err = repo.GetEventsByPaymentID("pay_1")
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to retrieve audit events")

	mock.ExpectQuery(`FROM payment_audit WHERE payment_id`).
		WillReturnRows(sqlmock.NewRows([]string{"event_type"}).AddRow("payment.authorized"))
	_, err = repo.GetEventsByPaymentID("pay_1")
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to scan audit event")

	mock.ExpectQuery(`FROM payment_audit WHERE payment_id`).
		WillReturnRows(sqlmock.NewRows([]string{"event_type", "details", "created_at"}))
	events, err = repo.GetEventsByPaymentID("pay_1")
	require.NoError(t, err)
	assert.Empty(t, events)
	assert.NoError(t, mock.ExpectationsWereMet())
}
