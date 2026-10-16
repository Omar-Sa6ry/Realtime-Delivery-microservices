package dynamodb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/domain"
	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/media-service/internal/testutil"
)

func newJobForTest(id string, status domain.JobStatus, createdAt time.Time) *domain.MediaJob {
	return &domain.MediaJob{
		JobID:       id,
		MediaID:     "m1",
		JobType:     domain.JobTypeCompress,
		Status:      status,
		MaxAttempts: 3,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}
}

func TestJobRepositoryCreate(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewJobRepository(newTestClient(t, f), testTable)

	if err := repo.Create(context.Background(), newJobForTest("j1", domain.JobStatusPending, fixedTime(5))); err != nil {
		t.Fatalf("Create: %v", err)
	}
	body := decodePutItem(t, lastTargetRequest(t, f, "PutItem").Body)
	cases := map[string]string{
		"PK":      "JOB#j1",
		"SK":      "JOB",
		"GSI2_PK": "JOB_STATUS#PENDING",
		"GSI2_SK": "CREATED#" + fixedTime(5).Format(time.RFC3339Nano),
		"JobID":   "j1",
		"MediaID": "m1",
		"JobType": "COMPRESS",
		"Status":  "PENDING",
	}
	for attr, want := range cases {
		if got := avString(t, body.Item, attr); got != want {
			t.Errorf("%s = %q, want %q", attr, got, want)
		}
	}
	if got := avNumber(t, body.Item, "MaxAttempts"); got != "3" {
		t.Errorf("MaxAttempts = %q", got)
	}
}

func TestJobRepositoryCreateError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("PutItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})
	if err := NewJobRepository(newTestClient(t, f), testTable).
		Create(context.Background(), newJobForTest("j1", domain.JobStatusPending, fixedTime(5))); err == nil {
		t.Fatal("expected error")
	}
}

func TestJobRepositoryGetByID(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewJobRepository(newTestClient(t, f), testTable)
	ctx := context.Background()
	j := newJobForTest("j1", domain.JobStatusRunning, fixedTime(5))
	j.Attempts = 1
	if err := repo.Create(ctx, j); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, "j1")
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.JobID != "j1" || got.MediaID != "m1" || got.JobType != domain.JobTypeCompress {
		t.Fatalf("unexpected job: %+v", got)
	}
	if got.Status != domain.JobStatusRunning || got.Attempts != 1 || got.MaxAttempts != 3 {
		t.Fatalf("unexpected job: %+v", got)
	}
	// Create does not persist LastError/StartedAt/CompletedAt (omitempty).
	if got.LastError != "" || !got.StartedAt.IsZero() || !got.CompletedAt.IsZero() {
		t.Fatalf("unexpected optional fields: %+v", got)
	}
	if !got.CreatedAt.Equal(fixedTime(5)) {
		t.Fatalf("unexpected timestamps: %+v", got)
	}
}

func TestJobRepositoryGetByIDNotFound(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	_, err := NewJobRepository(newTestClient(t, f), testTable).GetByID(context.Background(), "missing")
	if !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("err = %v, want ErrJobNotFound", err)
	}
}

func TestJobRepositoryGetByIDError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("GetItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})
	_, err := NewJobRepository(newTestClient(t, f), testTable).GetByID(context.Background(), "j1")
	if err == nil {
		t.Fatal("expected error")
	}
	requireContains(t, err.Error(), "DynamoDB GetItem job")
}

const jobBaseExpr = "SET #st = :st, GSI2_PK = :gsi2pk, UpdatedAt = :now, Attempts = Attempts + :one"

func TestJobRepositoryUpdateStatusBranches(t *testing.T) {
	tests := []struct {
		name         string
		status       domain.JobStatus
		lastError    string
		wantExpr     string
		wantErrValue string
	}{
		{
			name:     "running adds StartedAt",
			status:   domain.JobStatusRunning,
			wantExpr: jobBaseExpr + ", StartedAt = :now",
		},
		{
			name:     "completed adds CompletedAt",
			status:   domain.JobStatusCompleted,
			wantExpr: jobBaseExpr + ", CompletedAt = :now",
		},
		{
			name:         "failed with lastError",
			status:       domain.JobStatusFailed,
			lastError:    "disk full",
			wantExpr:     jobBaseExpr + ", LastError = :err" + ", CompletedAt = :now",
			wantErrValue: "disk full",
		},
		{
			name:     "failed without lastError",
			status:   domain.JobStatusFailed,
			wantExpr: jobBaseExpr + ", CompletedAt = :now",
		},
		{
			name:     "dlq adds neither",
			status:   domain.JobStatusDLQ,
			wantExpr: jobBaseExpr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := testutil.NewFakeDynamoDB(t)
			repo := NewJobRepository(newTestClient(t, f), testTable)
			ctx := context.Background()
			if err := repo.Create(ctx, newJobForTest("j1", domain.JobStatusPending, fixedTime(5))); err != nil {
				t.Fatalf("Create: %v", err)
			}

			if err := repo.UpdateStatus(ctx, "j1", tt.status, tt.lastError); err != nil {
				t.Fatalf("UpdateStatus: %v", err)
			}
			body := decodeUpdateItem(t, lastTargetRequest(t, f, "UpdateItem").Body)
			if body.UpdateExpression != tt.wantExpr {
				t.Errorf("UpdateExpression = %q, want %q", body.UpdateExpression, tt.wantExpr)
			}
			if got := avString(t, body.ExpressionAttributeValues, ":st"); got != string(tt.status) {
				t.Errorf(":st = %q", got)
			}
			if got := avString(t, body.ExpressionAttributeValues, ":gsi2pk"); got != "JOB_STATUS#"+string(tt.status) {
				t.Errorf(":gsi2pk = %q", got)
			}
			if got := avNumber(t, body.ExpressionAttributeValues, ":one"); got != "1" {
				t.Errorf(":one = %q", got)
			}
			if tt.wantErrValue != "" {
				if got := avString(t, body.ExpressionAttributeValues, ":err"); got != tt.wantErrValue {
					t.Errorf(":err = %q", got)
				}
			}

			got, err := repo.GetByID(ctx, "j1")
			if err != nil {
				t.Fatalf("GetByID: %v", err)
			}
			if got.Status != tt.status {
				t.Errorf("status = %q, want %q", got.Status, tt.status)
			}
		})
	}
}

func TestJobRepositoryUpdateStatusError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("UpdateItem", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})
	if err := NewJobRepository(newTestClient(t, f), testTable).
		UpdateStatus(context.Background(), "j1", domain.JobStatusRunning, ""); err == nil {
		t.Fatal("expected error")
	}
}

func TestJobRepositoryListStuck(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	repo := NewJobRepository(newTestClient(t, f), testTable)
	ctx := context.Background()

	started := time.Date(2026, 3, 1, 8, 0, 0, 0, time.UTC)
	if err := repo.Create(ctx, newJobForTest("j1", domain.JobStatusRunning, started)); err != nil {
		t.Fatalf("Create j1: %v", err)
	}
	if err := repo.Create(ctx, newJobForTest("j2", domain.JobStatusPending, started)); err != nil {
		t.Fatalf("Create j2: %v", err)
	}

	jobs, err := repo.ListStuck(ctx, started.Add(time.Hour))
	if err != nil {
		t.Fatalf("ListStuck: %v", err)
	}
	if len(jobs) != 1 || jobs[0].JobID != "j1" {
		t.Fatalf("stuck jobs = %v", jobs)
	}

	none, err := repo.ListStuck(ctx, started.Add(-time.Hour))
	if err != nil {
		t.Fatalf("ListStuck none: %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("expected no stuck jobs, got %v", none)
	}
}

func TestJobRepositoryListStuckError(t *testing.T) {
	f := testutil.NewFakeDynamoDB(t)
	f.SetError("Query", &testutil.DynamoError{Code: "ValidationException", Message: "bad", Status: 400})
	_, err := NewJobRepository(newTestClient(t, f), testTable).ListStuck(context.Background(), fixedTime(1))
	if err == nil {
		t.Fatal("expected error")
	}
	requireContains(t, err.Error(), "DynamoDB Query stuck jobs")
}
