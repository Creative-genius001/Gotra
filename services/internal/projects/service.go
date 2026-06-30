package projects

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"

	"github.com/gotra/gotra/pkg/security"
)

// ErrForbidden is returned when the user lacks the role for an action.
var ErrForbidden = errors.New("projects: insufficient permissions")

// Service implements project use cases with RBAC.
type Service struct {
	repo *Repository
}

// NewService constructs a project Service.
func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// List returns the caller's projects.
func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Project, error) {
	return s.repo.ListForUser(ctx, userID)
}

// Get returns a project the caller can access.
func (s *Service) Get(ctx context.Context, userID, projectID uuid.UUID) (*Project, error) {
	return s.repo.GetForUser(ctx, userID, projectID)
}

// Create makes a new project in the caller's personal workspace.
func (s *Service) Create(ctx context.Context, userID uuid.UUID, name string) (*Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Untitled Project"
	}
	workspaceID, err := s.repo.PrimaryWorkspaceID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.Create(ctx, userID, workspaceID, name, genSlug(name))
}

// Delete removes a project; only owners and admins may delete.
func (s *Service) Delete(ctx context.Context, userID, projectID uuid.UUID) error {
	role, err := s.repo.MemberRole(ctx, userID, projectID)
	if err != nil {
		return err
	}
	if role != string(security.RoleOwner) && role != string(security.RoleAdmin) {
		return ErrForbidden
	}
	return s.repo.Delete(ctx, projectID)
}

// genSlug builds a URL-safe, collision-resistant slug from a name.
func genSlug(name string) string {
	base := strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteByte('-')
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		slug = "project"
	}
	suffix, err := security.GenerateOpaqueToken(4)
	if err != nil {
		return slug + "-" + uuid.NewString()[:8]
	}
	return slug + "-" + strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(suffix))
}
