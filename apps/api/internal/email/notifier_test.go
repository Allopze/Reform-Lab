package email

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/allopze/reform-lab/apps/api/config"
	"github.com/allopze/reform-lab/apps/api/internal/domain"
	"github.com/allopze/reform-lab/apps/api/internal/queue"
	"github.com/allopze/reform-lab/apps/api/internal/repository"
	"github.com/google/uuid"
)

// --- Mocks for notifier tests ---

type spyQueue struct {
	emails []queue.EmailTaskPayload
}

func (q *spyQueue) Enqueue(_ context.Context, _ string, _ queue.TaskPayload, _ queue.TaskOptions) error {
	return nil
}
func (q *spyQueue) EnqueueEmail(_ context.Context, p queue.EmailTaskPayload, _ queue.TaskOptions) error {
	q.emails = append(q.emails, p)
	return nil
}
func (q *spyQueue) EnqueueWebhook(_ context.Context, _ queue.WebhookTaskPayload, _ queue.TaskOptions) error {
	return nil
}
func (q *spyQueue) Close() error { return nil }

type stubUserRepo struct {
	users map[uuid.UUID]*domain.User
}

func (r *stubUserRepo) Create(_ context.Context, u *domain.User) error {
	r.users[u.ID] = u
	return nil
}
func (r *stubUserRepo) GetByEmail(_ context.Context, email string) (*domain.User, error) {
	for _, u := range r.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, errors.New("not found")
}
func (r *stubUserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := r.users[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return u, nil
}
func (r *stubUserRepo) UpdatePasswordHash(_ context.Context, id uuid.UUID, passwordHash string) error {
	u, ok := r.users[id]
	if !ok || u == nil {
		return domain.ErrUserNotFound
	}
	u.PasswordHash = passwordHash
	return nil
}
func (r *stubUserRepo) UpdateEmailVerifiedAt(_ context.Context, id uuid.UUID, verifiedAt *time.Time) error {
	u, ok := r.users[id]
	if !ok || u == nil {
		return domain.ErrUserNotFound
	}
	u.EmailVerifiedAt = verifiedAt
	return nil
}
func (r *stubUserRepo) Count(_ context.Context) (int, error) { return len(r.users), nil }
func (r *stubUserRepo) HasAdmin(_ context.Context) (bool, error) {
	for _, u := range r.users {
		if u.Role == domain.RoleAdmin {
			return true, nil
		}
	}
	return false, nil
}
func (r *stubUserRepo) ListAll(_ context.Context) ([]domain.User, error) {
	out := make([]domain.User, 0, len(r.users))
	for _, u := range r.users {
		if u == nil {
			continue
		}
		out = append(out, *u)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}
func (r *stubUserRepo) ListForAdmin(_ context.Context, filter repository.AdminUserFilter) (*repository.AdminUserPage, error) {
	all, _ := r.ListAll(context.Background())
	if filter.Limit <= 0 {
		filter.Limit = len(all)
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	if filter.Offset >= len(all) {
		return &repository.AdminUserPage{Users: []domain.User{}, Total: len(all)}, nil
	}
	end := filter.Offset + filter.Limit
	if end > len(all) {
		end = len(all)
	}
	return &repository.AdminUserPage{Users: all[filter.Offset:end], Total: len(all)}, nil
}
func (r *stubUserRepo) UpdateRole(_ context.Context, id uuid.UUID, role domain.UserRole) error {
	u, ok := r.users[id]
	if !ok || u == nil {
		return domain.ErrUserNotFound
	}
	u.Role = role
	return nil
}
func (r *stubUserRepo) SetSuspended(_ context.Context, id uuid.UUID, suspended bool, reason *string) error {
	u, ok := r.users[id]
	if !ok || u == nil {
		return domain.ErrUserNotFound
	}
	u.IsSuspended = suspended
	u.SuspendedReason = reason
	return nil
}
func (r *stubUserRepo) RevokeSessions(_ context.Context, id uuid.UUID) (int, error) {
	u, ok := r.users[id]
	if !ok || u == nil {
		return 0, domain.ErrUserNotFound
	}
	if u.SessionVersion < 1 {
		u.SessionVersion = 1
	}
	u.SessionVersion++
	return u.SessionVersion, nil
}

type stubFileRepo struct {
	files map[uuid.UUID]*domain.OriginalFile
}

func (r *stubFileRepo) Create(_ context.Context, f *domain.OriginalFile) error {
	r.files[f.ID] = f
	return nil
}
func (r *stubFileRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.OriginalFile, error) {
	f, ok := r.files[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return f, nil
}
func (r *stubFileRepo) MarkExpiredByInternalName(_ context.Context, _ string) error { return nil }
func (r *stubFileRepo) DeleteExpiredBefore(_ context.Context, _ time.Time) (int64, error) {
	return 0, nil
}
func (r *stubFileRepo) CumulativeBytesByUser(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}
func (r *stubFileRepo) CumulativeBytesByGuestSession(_ context.Context, _ uuid.UUID) (int64, error) {
	return 0, nil
}

// --- Helpers ---

func smtpConfig() *config.Config {
	return &config.Config{
		SMTPHost: "mail.test.com",
		SMTPPort: 587,
		SMTPFrom: "noreply@test.com",
		AppURL:   "https://app.test.com",
	}
}

func noSMTPConfig() *config.Config {
	return &config.Config{AppURL: "https://app.test.com"}
}

func newNotifier(cfg *config.Config, q *spyQueue, users *stubUserRepo, files *stubFileRepo) *JobNotifier {
	settings := &mockSiteSettings{values: map[string]string{}}
	emailSvc := NewService(cfg, settings, nil, testLogger())
	return NewJobNotifier(cfg, emailSvc, q, users, files, testLogger())
}

func makeJob(userID *uuid.UUID, fileID uuid.UUID) *domain.Job {
	return &domain.Job{
		ID:           uuid.New(),
		UserID:       userID,
		FileID:       fileID,
		CapabilityID: "pdf-to-txt",
		OutputFormat: "txt",
		Status:       domain.JobSucceeded,
	}
}

// --- Tests ---

func TestNotifyJobCompleted_EnqueuesEmail(t *testing.T) {
	userID := uuid.New()
	fileID := uuid.New()
	q := &spyQueue{}
	users := &stubUserRepo{users: map[uuid.UUID]*domain.User{
		userID: {ID: userID, Name: "Ana", Email: "ana@test.com"},
	}}
	files := &stubFileRepo{files: map[uuid.UUID]*domain.OriginalFile{
		fileID: {ID: fileID, OriginalName: "report.pdf"},
	}}

	n := newNotifier(smtpConfig(), q, users, files)
	job := makeJob(&userID, fileID)

	if err := n.NotifyJobCompleted(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(q.emails) != 1 {
		t.Fatalf("expected 1 enqueued email, got %d", len(q.emails))
	}
	e := q.emails[0]
	if e.TemplateKey != "conversion-complete" {
		t.Errorf("expected template conversion-complete, got %s", e.TemplateKey)
	}
	if e.To != "ana@test.com" {
		t.Errorf("expected to ana@test.com, got %s", e.To)
	}
	if e.Vars["FileName"] != "report.pdf" {
		t.Errorf("expected FileName report.pdf, got %s", e.Vars["FileName"])
	}
	if e.Vars["OutputFormat"] != "txt" {
		t.Errorf("expected OutputFormat txt, got %s", e.Vars["OutputFormat"])
	}
	if e.Vars["AppURL"] != "https://app.test.com" {
		t.Errorf("expected AppURL https://app.test.com, got %s", e.Vars["AppURL"])
	}
	if _, ok := e.Vars["ErrorMessage"]; ok {
		t.Error("ErrorMessage should not be present on success")
	}
}

func TestNotifyJobFailed_EnqueuesEmailWithError(t *testing.T) {
	userID := uuid.New()
	fileID := uuid.New()
	q := &spyQueue{}
	users := &stubUserRepo{users: map[uuid.UUID]*domain.User{
		userID: {ID: userID, Name: "Bob", Email: "bob@test.com"},
	}}
	files := &stubFileRepo{files: map[uuid.UUID]*domain.OriginalFile{
		fileID: {ID: fileID, OriginalName: "photo.png"},
	}}

	n := newNotifier(smtpConfig(), q, users, files)
	errStr := "engine timeout"
	job := makeJob(&userID, fileID)
	job.Status = domain.JobFailed
	job.Error = &errStr

	if err := n.NotifyJobFailed(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(q.emails) != 1 {
		t.Fatalf("expected 1 enqueued email, got %d", len(q.emails))
	}
	e := q.emails[0]
	if e.TemplateKey != "conversion-failed" {
		t.Errorf("expected template conversion-failed, got %s", e.TemplateKey)
	}
	if e.Vars["ErrorMessage"] != "engine timeout" {
		t.Errorf("expected ErrorMessage 'engine timeout', got %s", e.Vars["ErrorMessage"])
	}
}

func TestNotifyJobFailed_DefaultErrorMessage(t *testing.T) {
	userID := uuid.New()
	fileID := uuid.New()
	q := &spyQueue{}
	users := &stubUserRepo{users: map[uuid.UUID]*domain.User{
		userID: {ID: userID, Name: "Carlos", Email: "carlos@test.com"},
	}}
	files := &stubFileRepo{files: map[uuid.UUID]*domain.OriginalFile{
		fileID: {ID: fileID, OriginalName: "doc.docx"},
	}}

	n := newNotifier(smtpConfig(), q, users, files)
	job := makeJob(&userID, fileID)
	job.Status = domain.JobFailed
	job.Error = nil // no error message set

	if err := n.NotifyJobFailed(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if q.emails[0].Vars["ErrorMessage"] != "error desconocido" {
		t.Errorf("expected default error message, got %s", q.emails[0].Vars["ErrorMessage"])
	}
}

func TestNotify_SkipsGuestJob(t *testing.T) {
	fileID := uuid.New()
	q := &spyQueue{}
	users := &stubUserRepo{users: map[uuid.UUID]*domain.User{}}
	files := &stubFileRepo{files: map[uuid.UUID]*domain.OriginalFile{
		fileID: {ID: fileID, OriginalName: "file.pdf"},
	}}

	n := newNotifier(smtpConfig(), q, users, files)
	job := makeJob(nil, fileID) // guest — no UserID

	if err := n.NotifyJobCompleted(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(q.emails) != 0 {
		t.Fatalf("expected 0 enqueued emails for guest job, got %d", len(q.emails))
	}
}

func TestNotify_SkipsWhenSMTPNotConfigured(t *testing.T) {
	userID := uuid.New()
	fileID := uuid.New()
	q := &spyQueue{}
	users := &stubUserRepo{users: map[uuid.UUID]*domain.User{
		userID: {ID: userID, Name: "Diana", Email: "diana@test.com"},
	}}
	files := &stubFileRepo{files: map[uuid.UUID]*domain.OriginalFile{
		fileID: {ID: fileID, OriginalName: "file.pdf"},
	}}

	n := newNotifier(noSMTPConfig(), q, users, files) // no SMTP host
	job := makeJob(&userID, fileID)

	if err := n.NotifyJobCompleted(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(q.emails) != 0 {
		t.Fatalf("expected 0 enqueued emails without SMTP, got %d", len(q.emails))
	}
}

func TestNotify_GracefulOnMissingUser(t *testing.T) {
	userID := uuid.New()
	fileID := uuid.New()
	q := &spyQueue{}
	users := &stubUserRepo{users: map[uuid.UUID]*domain.User{}} // user not found
	files := &stubFileRepo{files: map[uuid.UUID]*domain.OriginalFile{
		fileID: {ID: fileID, OriginalName: "file.pdf"},
	}}

	n := newNotifier(smtpConfig(), q, users, files)
	job := makeJob(&userID, fileID)

	// Should not error — just skip gracefully.
	if err := n.NotifyJobCompleted(context.Background(), job); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(q.emails) != 0 {
		t.Fatalf("expected 0 emails when user missing, got %d", len(q.emails))
	}
}
