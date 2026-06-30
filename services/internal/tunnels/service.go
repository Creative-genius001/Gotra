package tunnels

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/gotra/gotra/pkg/security"
)

// ErrForbidden is returned when the user lacks the role for an action.
var ErrForbidden = errors.New("tunnels: insufficient permissions")

// tunnelBaseDomain is the suffix used to mint public tunnel URLs. The gateway
// will route these once the Agent↔Gateway protocol is implemented.
const tunnelBaseDomain = "tunnels.gotra.local"

// QuotaChecker enforces per-plan limits before a tunnel is created. It is
// satisfied by the billing service; nil disables quota enforcement.
type QuotaChecker interface {
	CheckTunnelQuota(ctx context.Context, userID uuid.UUID) error
}

// Service implements tunnel use cases with project-scoped RBAC.
type Service struct {
	repo  *Repository
	quota QuotaChecker
}

// NewService constructs a tunnel Service.
func NewService(repo *Repository, quota QuotaChecker) *Service {
	return &Service{repo: repo, quota: quota}
}

// List returns tunnels for a project the caller can access.
func (s *Service) List(ctx context.Context, userID, projectID uuid.UUID) ([]Tunnel, error) {
	if _, err := s.repo.ProjectRole(ctx, userID, projectID); err != nil {
		return nil, err // ErrNotFound when not a member
	}
	return s.repo.ListForProject(ctx, projectID)
}

// Create provisions a tunnel for a project. Viewers may not create tunnels.
func (s *Service) Create(ctx context.Context, userID, projectID uuid.UUID, localPort int) (*Tunnel, error) {
	if localPort <= 0 || localPort > 65535 {
		return nil, fmt.Errorf("invalid local port")
	}
	role, err := s.repo.ProjectRole(ctx, userID, projectID)
	if err != nil {
		return nil, err
	}
	if role == string(security.RoleViewer) {
		return nil, ErrForbidden
	}
	if s.quota != nil {
		if err := s.quota.CheckTunnelQuota(ctx, userID); err != nil {
			return nil, err
		}
	}
	return s.repo.Create(ctx, projectID, genPublicURL(), localPort)
}

// Get returns a tunnel the caller can access.
func (s *Service) Get(ctx context.Context, userID, tunnelID uuid.UUID) (*Tunnel, error) {
	t, _, err := s.repo.GetWithAccess(ctx, userID, tunnelID)
	return t, err
}

// Delete removes a tunnel. Viewers may not delete tunnels.
func (s *Service) Delete(ctx context.Context, userID, tunnelID uuid.UUID) error {
	_, role, err := s.repo.GetWithAccess(ctx, userID, tunnelID)
	if err != nil {
		return err
	}
	if role == string(security.RoleViewer) {
		return ErrForbidden
	}
	return s.repo.Delete(ctx, tunnelID)
}

// genPublicURL mints a unique public URL for a new tunnel.
func genPublicURL() string {
	sub, err := security.GenerateOpaqueToken(6)
	if err != nil {
		sub = uuid.NewString()[:12]
	}
	sub = strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(sub))
	return fmt.Sprintf("https://%s.%s", sub, tunnelBaseDomain)
}
