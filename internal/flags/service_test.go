package flags

import (
	"context"
	"testing"
	"time"

	"github.com/planatechnologies/goldpath/internal/observability"
)

// Global metrics instance - created once to avoid duplicate registration
var testMetrics = observability.NewMetrics()

// MockRepository is a mock implementation of Repository for testing
type MockRepository struct {
	flags     map[string]*FeatureFlag
	GetErr    error
	ListErr   error
	CreateErr error
	UpdateErr error
	DeleteErr error
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		flags: make(map[string]*FeatureFlag),
	}
}

func (m *MockRepository) Get(ctx context.Context, key string) (*FeatureFlag, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	flag, exists := m.flags[key]
	if !exists {
		return nil, ErrFlagNotFound
	}
	return flag, nil
}

func (m *MockRepository) List(ctx context.Context) ([]*FeatureFlag, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}
	flags := make([]*FeatureFlag, 0, len(m.flags))
	for _, flag := range m.flags {
		flags = append(flags, flag)
	}
	return flags, nil
}

func (m *MockRepository) Create(ctx context.Context, flag *FeatureFlag) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	if _, exists := m.flags[flag.Key]; exists {
		return ErrFlagAlreadyExists
	}
	flag.CreatedAt = time.Now()
	flag.UpdatedAt = time.Now()
	m.flags[flag.Key] = flag
	return nil
}

func (m *MockRepository) Update(ctx context.Context, flag *FeatureFlag) error {
	if m.UpdateErr != nil {
		return m.UpdateErr
	}
	existing, exists := m.flags[flag.Key]
	if !exists {
		return ErrFlagNotFound
	}
	flag.ID = existing.ID
	flag.CreatedAt = existing.CreatedAt
	flag.UpdatedAt = time.Now()
	m.flags[flag.Key] = flag
	return nil
}

func (m *MockRepository) Delete(ctx context.Context, key string) error {
	if m.DeleteErr != nil {
		return m.DeleteErr
	}
	if _, exists := m.flags[key]; !exists {
		return ErrFlagNotFound
	}
	delete(m.flags, key)
	return nil
}

func newTestFlag(key string, enabled bool, rollout float64) *FeatureFlag {
	now := time.Now()
	return &FeatureFlag{
		ID:          "test-id-" + key,
		Key:         key,
		Name:        "Test Flag " + key,
		Description: "Test description for " + key,
		Enabled:     enabled,
		Rollout:     rollout,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func TestInMemoryRepository_Get(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name        string
		key         string
		setupFlags  []*FeatureFlag
		wantErr     error
	}{
		{"flag exists", "test-flag", []*FeatureFlag{newTestFlag("test-flag", true, 100.0)}, nil},
		{"flag not found", "nonexistent", []*FeatureFlag{}, ErrFlagNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewInMemoryRepository()
			for _, f := range tt.setupFlags {
				repo.flags[f.Key] = f
			}
			got, err := repo.Get(ctx, tt.key)
			if tt.wantErr != nil {
				if err != tt.wantErr {
					t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("Get() unexpected error = %v", err)
			}
			if got != nil && got.Key != tt.key {
				t.Errorf("Get().Key = %v, want %v", got.Key, tt.key)
			}
		})
	}
}

func TestInMemoryRepository_List(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()
	got, err := repo.List(ctx)
	if err != nil {
		t.Errorf("List() unexpected error = %v", err)
	}
	if len(got) < 1 {
		t.Errorf("List() length = %v, want at least 1", len(got))
	}
}

func TestInMemoryRepository_Create(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()
	repo.flags = make(map[string]*FeatureFlag)
	flag := newTestFlag("new-flag", true, 100.0)
	err := repo.Create(ctx, flag)
	if err != nil {
		t.Errorf("Create() error = %v", err)
	}
	if repo.flags["new-flag"] == nil {
		t.Errorf("Create() flag not found")
	}
}

func TestInMemoryRepository_Update(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()
	repo.flags = make(map[string]*FeatureFlag)
	repo.flags["test-flag"] = newTestFlag("test-flag", true, 100.0)
	err := repo.Update(ctx, &FeatureFlag{Key: "test-flag", Enabled: false, Rollout: 50.0})
	if err != nil {
		t.Errorf("Update() error = %v", err)
	}
	if repo.flags["test-flag"].Enabled != false {
		t.Errorf("Update().Enabled = %v, want false", repo.flags["test-flag"].Enabled)
	}
}

func TestInMemoryRepository_Delete(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()
	repo.flags = make(map[string]*FeatureFlag)
	repo.flags["test-flag"] = newTestFlag("test-flag", true, 100.0)
	err := repo.Delete(ctx, "test-flag")
	if err != nil {
		t.Errorf("Delete() error = %v", err)
	}
	if _, exists := repo.flags["test-flag"]; exists {
		t.Errorf("Delete() flag still exists")
	}
}

func TestService_GetFlag(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.flags["test-flag"] = newTestFlag("test-flag", true, 100.0)
	svc := NewService(repo, testMetrics)
	got, err := svc.GetFlag(ctx, "test-flag")
	if err != nil {
		t.Errorf("GetFlag() error = %v", err)
	}
	if got.Key != "test-flag" {
		t.Errorf("GetFlag().Key = %v, want test-flag", got.Key)
	}
}

func TestService_GetFlag_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewService(repo, testMetrics)
	_, err := svc.GetFlag(ctx, "nonexistent")
	if err != ErrFlagNotFound {
		t.Errorf("GetFlag() error = %v, want ErrFlagNotFound", err)
	}
}

func TestService_ListFlags(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.flags["flag1"] = newTestFlag("flag1", true, 100.0)
	svc := NewService(repo, testMetrics)
	got, err := svc.ListFlags(ctx)
	if err != nil {
		t.Errorf("ListFlags() error = %v", err)
	}
	if len(got) != 1 {
		t.Errorf("ListFlags() length = %v, want 1", len(got))
	}
}

func TestService_CreateFlag(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewService(repo, testMetrics)
	flag := newTestFlag("new-flag", true, 100.0)
	err := svc.CreateFlag(ctx, flag)
	if err != nil {
		t.Errorf("CreateFlag() error = %v", err)
	}
}

func TestService_CreateFlag_AlreadyExists(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.flags["existing"] = newTestFlag("existing", true, 100.0)
	svc := NewService(repo, testMetrics)
	err := svc.CreateFlag(ctx, newTestFlag("existing", true, 100.0))
	if err != ErrFlagAlreadyExists {
		t.Errorf("CreateFlag() error = %v, want ErrFlagAlreadyExists", err)
	}
}

func TestService_UpdateFlag(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.flags["test-flag"] = newTestFlag("test-flag", true, 100.0)
	svc := NewService(repo, testMetrics)
	err := svc.UpdateFlag(ctx, &FeatureFlag{Key: "test-flag", Enabled: false, Rollout: 50.0})
	if err != nil {
		t.Errorf("UpdateFlag() error = %v", err)
	}
}

func TestService_DeleteFlag(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.flags["test-flag"] = newTestFlag("test-flag", true, 100.0)
	svc := NewService(repo, testMetrics)
	err := svc.DeleteFlag(ctx, "test-flag")
	if err != nil {
		t.Errorf("DeleteFlag() error = %v", err)
	}
}

func TestService_IsEnabled(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.flags["test-flag"] = newTestFlag("test-flag", true, 100.0)
	svc := NewService(repo, testMetrics)
	got, err := svc.IsEnabled(ctx, "test-flag")
	if err != nil {
		t.Errorf("IsEnabled() error = %v", err)
	}
	if got != true {
		t.Errorf("IsEnabled() = %v, want true", got)
	}
}

func TestService_IsEnabled_Disabled(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.flags["test-flag"] = newTestFlag("test-flag", false, 100.0)
	svc := NewService(repo, testMetrics)
	got, err := svc.IsEnabled(ctx, "test-flag")
	if err != nil {
		t.Errorf("IsEnabled() error = %v", err)
	}
	if got != false {
		t.Errorf("IsEnabled() = %v, want false", got)
	}
}

func TestService_IsEnabled_WithUserID(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.flags["test-flag"] = newTestFlag("test-flag", true, 100.0)
	svc := NewService(repo, testMetrics)
	got, err := svc.IsEnabled(ctx, "test-flag", "user-123")
	if err != nil {
		t.Errorf("IsEnabled() error = %v", err)
	}
	if got != true {
		t.Errorf("IsEnabled() = %v, want true", got)
	}
}

func TestService_IsEnabled_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewService(repo, testMetrics)
	_, err := svc.IsEnabled(ctx, "nonexistent")
	if err != ErrFlagNotFound {
		t.Errorf("IsEnabled() error = %v, want ErrFlagNotFound", err)
	}
}

func TestService_ToggleFlag(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	repo.flags["test-flag"] = newTestFlag("test-flag", true, 100.0)
	svc := NewService(repo, testMetrics)
	flag, err := svc.ToggleFlag(ctx, "test-flag")
	if err != nil {
		t.Errorf("ToggleFlag() error = %v", err)
	}
	if flag.Enabled != false {
		t.Errorf("ToggleFlag().Enabled = %v, want false", flag.Enabled)
	}
}

func TestService_ToggleFlag_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewMockRepository()
	svc := NewService(repo, testMetrics)
	_, err := svc.ToggleFlag(ctx, "nonexistent")
	if err != ErrFlagNotFound {
		t.Errorf("ToggleFlag() error = %v, want ErrFlagNotFound", err)
	}
}

func TestFnv32a(t *testing.T) {
	hash1 := fnv32a("test")
	hash2 := fnv32a("test")
	if hash1 != hash2 {
		t.Errorf("fnv32a() not deterministic")
	}
	if hash1 == 0 {
		t.Errorf("fnv32a() returned 0")
	}
}

func TestService_UpdatePreservesMetadata(t *testing.T) {
	ctx := context.Background()
	originalTime := time.Now().Add(-24 * time.Hour)
	repo := NewMockRepository()
	repo.flags["test-flag"] = &FeatureFlag{
		ID:        "original-id",
		Key:       "test-flag",
		Name:      "Original Name",
		Enabled:   true,
		Rollout:  100.0,
		CreatedAt: originalTime,
		UpdatedAt: originalTime,
	}
	svc := NewService(repo, testMetrics)
	err := svc.UpdateFlag(ctx, &FeatureFlag{Key: "test-flag", Name: "Updated Name", Enabled: false, Rollout: 50.0})
	if err != nil {
		t.Fatalf("UpdateFlag() unexpected error = %v", err)
	}
	flag := repo.flags["test-flag"]
	if flag.ID != "original-id" {
		t.Errorf("UpdateFlag() did not preserve ID")
	}
}

func TestService_ToggleFlag_UpdatesTimestamp(t *testing.T) {
	ctx := context.Background()
	originalTime := time.Now().Add(-24 * time.Hour)
	repo := NewMockRepository()
	repo.flags["test-flag"] = &FeatureFlag{
		ID:        "test-id",
		Key:       "test-flag",
		Name:      "Test Flag",
		Enabled:   true,
		Rollout:  100.0,
		CreatedAt: originalTime,
		UpdatedAt: originalTime,
	}
	svc := NewService(repo, testMetrics)
	flag, err := svc.ToggleFlag(ctx, "test-flag")
	if err != nil {
		t.Fatalf("ToggleFlag() unexpected error = %v", err)
	}
	if !flag.UpdatedAt.After(originalTime) {
		t.Errorf("ToggleFlag() did not update UpdatedAt timestamp")
	}
}
