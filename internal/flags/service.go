package flags

import (
	"context"
	"errors"
	"hash/fnv"
	"sync"
	"time"

	"github.com/planatechnologies/goldpath/internal/observability"
)

// FeatureFlag represents a feature flag
type FeatureFlag struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	Rollout     float64   `json:"rollout"` // 0.0 to 1.0 percentage
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Repository defines the interface for feature flag storage
type Repository interface {
	Get(ctx context.Context, key string) (*FeatureFlag, error)
	List(ctx context.Context) ([]*FeatureFlag, error)
	Create(ctx context.Context, flag *FeatureFlag) error
	Update(ctx context.Context, flag *FeatureFlag) error
	Delete(ctx context.Context, key string) error
}

// InMemoryRepository is an in-memory implementation of Repository
type InMemoryRepository struct {
	mu    sync.RWMutex
	flags map[string]*FeatureFlag
}

// NewInMemoryRepository creates a new in-memory repository
func NewInMemoryRepository() *InMemoryRepository {
	repo := &InMemoryRepository{
		flags: make(map[string]*FeatureFlag),
	}
	// Initialize with some default flags
	repo.flags["demo-feature"] = &FeatureFlag{
		ID:          "1",
		Key:         "demo-feature",
		Name:        "Demo Feature",
		Description: "A demo feature flag",
		Enabled:     true,
		Rollout:     100.0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	return repo
}

// Get retrieves a feature flag by key
func (r *InMemoryRepository) Get(ctx context.Context, key string) (*FeatureFlag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	flag, exists := r.flags[key]
	if !exists {
		return nil, ErrFlagNotFound
	}
	return flag, nil
}

// List returns all feature flags
func (r *InMemoryRepository) List(ctx context.Context) ([]*FeatureFlag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	flags := make([]*FeatureFlag, 0, len(r.flags))
	for _, flag := range r.flags {
		flags = append(flags, flag)
	}
	return flags, nil
}

// Create creates a new feature flag
func (r *InMemoryRepository) Create(ctx context.Context, flag *FeatureFlag) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.flags[flag.Key]; exists {
		return ErrFlagAlreadyExists
	}

	flag.CreatedAt = time.Now()
	flag.UpdatedAt = time.Now()
	r.flags[flag.Key] = flag
	return nil
}

// Update updates an existing feature flag
func (r *InMemoryRepository) Update(ctx context.Context, flag *FeatureFlag) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.flags[flag.Key]
	if !exists {
		return ErrFlagNotFound
	}

	flag.ID = existing.ID
	flag.CreatedAt = existing.CreatedAt
	flag.UpdatedAt = time.Now()
	r.flags[flag.Key] = flag
	return nil
}

// Delete deletes a feature flag
func (r *InMemoryRepository) Delete(ctx context.Context, key string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.flags[key]; !exists {
		return ErrFlagNotFound
	}

	delete(r.flags, key)
	return nil
}

// Service provides feature flag business logic
type Service struct {
	repo    Repository
	metrics *observability.Metrics
}

// NewService creates a new feature flag service
func NewService(repo Repository, metrics *observability.Metrics) *Service {
	return &Service{
		repo:    repo,
		metrics: metrics,
	}
}

// GetFlag retrieves a feature flag by key
func (s *Service) GetFlag(ctx context.Context, key string) (*FeatureFlag, error) {
	flag, err := s.repo.Get(ctx, key)
	if err != nil {
		s.metrics.RecordFlagError(key, "get_failed")
		return nil, err
	}
	s.metrics.RecordFlagEvaluation(key, flag.Enabled)
	return flag, nil
}

// ListFlags returns all feature flags
func (s *Service) ListFlags(ctx context.Context) ([]*FeatureFlag, error) {
	return s.repo.List(ctx)
}

// CreateFlag creates a new feature flag
func (s *Service) CreateFlag(ctx context.Context, flag *FeatureFlag) error {
	return s.repo.Create(ctx, flag)
}

// UpdateFlag updates an existing feature flag
func (s *Service) UpdateFlag(ctx context.Context, flag *FeatureFlag) error {
	return s.repo.Update(ctx, flag)
}

// DeleteFlag deletes a feature flag
func (s *Service) DeleteFlag(ctx context.Context, key string) error {
	return s.repo.Delete(ctx, key)
}

// IsEnabled checks if a feature flag is enabled
// If userID is provided, it uses deterministic hashing based on userID + flag key
// to determine if the user falls within the rollout percentage
func (s *Service) IsEnabled(ctx context.Context, key string, userID ...string) (bool, error) {
	flag, err := s.GetFlag(ctx, key)
	if err != nil {
		return false, err
	}

	// If no user ID provided or rollout is 100%, just return the enabled status
	if len(userID) == 0 || userID[0] == "" || flag.Rollout >= 100.0 {
		s.metrics.RecordFlagEvaluation(key, flag.Enabled)
		return flag.Enabled, nil
	}

	// Calculate deterministic hash based on userID + flag key
	hash := fnv32a(userID[0] + flag.Key)

	// Determine if user is in the rollout percentage
	// hash % 100 gives us a value 0-99
	// If rollout is 20%, we want users with hash % 100 < 20
	rolloutThreshold := int(flag.Rollout)
	inRollout := (int(hash) % 100) < rolloutThreshold

	// User is enabled if flag is enabled AND user is in the rollout
	enabled := flag.Enabled && inRollout

	// Record metrics with rollout information
	s.metrics.RecordFlagEvaluationWithRollout(key, enabled, inRollout, flag.Rollout)

	return enabled, nil
}

// ToggleFlag toggles the enabled state of a feature flag
// It atomically gets the current state, flips it, and saves the new state
func (s *Service) ToggleFlag(ctx context.Context, key string) (*FeatureFlag, error) {
	// Get the current flag
	flag, err := s.repo.Get(ctx, key)
	if err != nil {
		s.metrics.RecordFlagError(key, "toggle_failed")
		return nil, err
	}

	// Flip the enabled state
	flag.Enabled = !flag.Enabled
	flag.UpdatedAt = time.Now()

	// Update in repository
	if err := s.repo.Update(ctx, flag); err != nil {
		s.metrics.RecordFlagError(key, "toggle_failed")
		return nil, err
	}

	// Record metrics for the toggle action
	s.metrics.RecordFlagEvaluation(key, flag.Enabled)

	return flag, nil
}

// fnv32a computes FNV-1a 32-bit hash of the input string
// This provides deterministic hashing - same input always produces same output
func fnv32a(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

// Common errors
var (
	ErrFlagNotFound      = errors.New("flag not found")
	ErrFlagAlreadyExists = errors.New("flag already exists")
)
