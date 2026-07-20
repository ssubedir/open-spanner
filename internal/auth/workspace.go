package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/ssubedir/open-spanner/internal/metering/domain"
)

const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleViewer = "viewer"

	defaultInvitationTTL  = 7 * 24 * time.Hour
	invitationTokenPrefix = "osp_wsi_"
)

type CreateWorkspaceInvitationCommand struct {
	Email string
	Role  string
}

type UpdateWorkspaceMemberCommand struct {
	UserID string
	Role   string
}

type WorkspaceInvitationResult struct {
	WorkspaceInvitation
	Token  string
	Status string
}

func (s Service) ListWorkspaces(ctx context.Context, userID string) ([]WorkspaceAccess, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.Join(domain.ErrInvalidInput, errors.New("user id is required"))
	}
	return s.repo.ListWorkspaceAccessByUserID(ctx, userID)
}

func (s Service) SwitchWorkspace(ctx context.Context, userID string, workspaceID string) (LoginResult, error) {
	userID = strings.TrimSpace(userID)
	workspaceID = strings.TrimSpace(workspaceID)
	if userID == "" || workspaceID == "" {
		return LoginResult{}, errors.Join(domain.ErrInvalidInput, errors.New("workspace id is required"))
	}
	user, err := s.repo.FindUserByID(ctx, userID)
	if err != nil {
		return LoginResult{}, err
	}
	return s.createLoginResultForWorkspace(ctx, user, workspaceID)
}

func (s Service) ListWorkspaceMembers(ctx context.Context, principal Principal) ([]WorkspaceMember, error) {
	if err := requireWorkspaceRole(principal, RoleOwner, RoleAdmin); err != nil {
		return nil, err
	}
	return s.repo.ListWorkspaceMembers(ctx, principal.WorkspaceID)
}

func (s Service) ListWorkspaceInvitations(ctx context.Context, principal Principal) ([]WorkspaceInvitationResult, error) {
	if err := requireWorkspaceRole(principal, RoleOwner, RoleAdmin); err != nil {
		return nil, err
	}
	invitations, err := s.repo.ListWorkspaceInvitations(ctx, principal.WorkspaceID)
	if err != nil {
		return nil, err
	}
	results := make([]WorkspaceInvitationResult, 0, len(invitations))
	for _, invitation := range invitations {
		results = append(results, WorkspaceInvitationResult{WorkspaceInvitation: invitation, Status: invitationStatus(invitation, s.now().UTC())})
	}
	return results, nil
}

func (s Service) CreateWorkspaceInvitation(ctx context.Context, principal Principal, cmd CreateWorkspaceInvitationCommand) (WorkspaceInvitationResult, error) {
	if err := requireWorkspaceRole(principal, RoleOwner, RoleAdmin); err != nil {
		return WorkspaceInvitationResult{}, err
	}
	email, err := normalizeEmail(cmd.Email)
	if err != nil {
		return WorkspaceInvitationResult{}, err
	}
	role, err := normalizeInvitableRole(cmd.Role)
	if err != nil {
		return WorkspaceInvitationResult{}, err
	}
	if user, findErr := s.repo.FindUserByEmail(ctx, email); findErr == nil {
		if _, accessErr := s.repo.FindWorkspaceAccess(ctx, principal.WorkspaceID, user.ID); accessErr == nil {
			return WorkspaceInvitationResult{}, errors.Join(domain.ErrConflict, errors.New("user is already a workspace member"))
		} else if !errors.Is(accessErr, domain.ErrNotFound) {
			return WorkspaceInvitationResult{}, accessErr
		}
	} else if !errors.Is(findErr, domain.ErrNotFound) {
		return WorkspaceInvitationResult{}, findErr
	}
	existingInvitations, err := s.repo.ListWorkspaceInvitations(ctx, principal.WorkspaceID)
	if err != nil {
		return WorkspaceInvitationResult{}, err
	}
	for _, existing := range existingInvitations {
		if !strings.EqualFold(existing.Email, email) || existing.AcceptedAt != nil || existing.RevokedAt != nil {
			continue
		}
		if existing.ExpiresAt.After(s.now().UTC()) {
			return WorkspaceInvitationResult{}, errors.Join(domain.ErrConflict, errors.New("an active invitation already exists for this email"))
		}
		if err := s.repo.RevokeWorkspaceInvitation(ctx, principal.WorkspaceID, existing.ID, s.now().UTC()); err != nil {
			return WorkspaceInvitationResult{}, err
		}
	}
	token, err := newSessionToken(invitationTokenPrefix, s.tokenBytes)
	if err != nil {
		return WorkspaceInvitationResult{}, err
	}
	now := s.now().UTC()
	invitation := WorkspaceInvitation{
		ID: uuid.NewString(), WorkspaceID: principal.WorkspaceID, WorkspaceName: principal.User.WorkspaceName,
		Email: email, Role: role, TokenHash: HashToken(token), InvitedByUserID: principal.User.ID,
		ExpiresAt: now.Add(defaultInvitationTTL), CreatedAt: now,
	}
	invitation, err = s.repo.SaveWorkspaceInvitation(ctx, invitation)
	if err != nil {
		return WorkspaceInvitationResult{}, err
	}
	return WorkspaceInvitationResult{WorkspaceInvitation: invitation, Token: token, Status: "pending"}, nil
}

func (s Service) PreviewWorkspaceInvitation(ctx context.Context, token string) (WorkspaceInvitationResult, error) {
	invitation, err := s.invitationByToken(ctx, token)
	if err != nil {
		return WorkspaceInvitationResult{}, err
	}
	return WorkspaceInvitationResult{WorkspaceInvitation: invitation, Status: invitationStatus(invitation, s.now().UTC())}, nil
}

func (s Service) AcceptWorkspaceInvitation(ctx context.Context, principal Principal, token string) (LoginResult, error) {
	invitation, err := s.invitationByToken(ctx, token)
	if err != nil {
		return LoginResult{}, err
	}
	if invitationStatus(invitation, s.now().UTC()) != "pending" {
		return LoginResult{}, errors.Join(domain.ErrConflict, errors.New("workspace invitation is no longer active"))
	}
	if !strings.EqualFold(invitation.Email, principal.User.Email) {
		return LoginResult{}, errors.Join(domain.ErrForbidden, errors.New("workspace invitation belongs to another email address"))
	}
	if _, err := s.repo.FindWorkspaceAccess(ctx, invitation.WorkspaceID, principal.User.ID); err == nil {
		return LoginResult{}, errors.Join(domain.ErrConflict, errors.New("user is already a workspace member"))
	} else if !errors.Is(err, domain.ErrNotFound) {
		return LoginResult{}, err
	}
	now := s.now().UTC()
	err = s.repo.AcceptWorkspaceInvitation(ctx, invitation, WorkspaceMembership{
		WorkspaceID: invitation.WorkspaceID, UserID: principal.User.ID, Role: invitation.Role, CreatedAt: now,
	}, now)
	if err != nil {
		return LoginResult{}, err
	}
	user, err := s.repo.FindUserByID(ctx, principal.User.ID)
	if err != nil {
		return LoginResult{}, err
	}
	return s.createLoginResultForWorkspace(ctx, user, invitation.WorkspaceID)
}

func (s Service) RevokeWorkspaceInvitation(ctx context.Context, principal Principal, id string) error {
	if err := requireWorkspaceRole(principal, RoleOwner, RoleAdmin); err != nil {
		return err
	}
	return s.repo.RevokeWorkspaceInvitation(ctx, principal.WorkspaceID, strings.TrimSpace(id), s.now().UTC())
}

func (s Service) UpdateWorkspaceMember(ctx context.Context, principal Principal, cmd UpdateWorkspaceMemberCommand) error {
	if err := requireWorkspaceRole(principal, RoleOwner); err != nil {
		return err
	}
	role, err := normalizeMemberRole(cmd.Role)
	if err != nil {
		return err
	}
	access, err := s.repo.FindWorkspaceAccess(ctx, principal.WorkspaceID, strings.TrimSpace(cmd.UserID))
	if err != nil {
		return err
	}
	if access.Membership.Role == RoleOwner && role != RoleOwner {
		owners, err := s.repo.CountWorkspaceOwners(ctx, principal.WorkspaceID)
		if err != nil {
			return err
		}
		if owners <= 1 {
			return errors.Join(domain.ErrConflict, errors.New("cannot demote the final workspace owner"))
		}
	}
	return s.repo.UpdateWorkspaceMembershipRole(ctx, principal.WorkspaceID, cmd.UserID, role)
}

func (s Service) RemoveWorkspaceMember(ctx context.Context, principal Principal, userID string) error {
	if err := requireWorkspaceRole(principal, RoleOwner); err != nil {
		return err
	}
	access, err := s.repo.FindWorkspaceAccess(ctx, principal.WorkspaceID, strings.TrimSpace(userID))
	if err != nil {
		return err
	}
	if access.Membership.Role == RoleOwner {
		owners, err := s.repo.CountWorkspaceOwners(ctx, principal.WorkspaceID)
		if err != nil {
			return err
		}
		if owners <= 1 {
			return errors.Join(domain.ErrConflict, errors.New("cannot remove the final workspace owner"))
		}
	}
	return s.repo.DeleteWorkspaceMembership(ctx, principal.WorkspaceID, userID)
}

func (s Service) invitationByToken(ctx context.Context, token string) (WorkspaceInvitation, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return WorkspaceInvitation{}, errors.Join(domain.ErrInvalidInput, errors.New("invitation token is required"))
	}
	return s.repo.FindWorkspaceInvitationByTokenHash(ctx, HashToken(token))
}

func requireWorkspaceRole(principal Principal, roles ...string) error {
	for _, role := range roles {
		if principal.Role == role {
			return nil
		}
	}
	return errors.Join(domain.ErrForbidden, errors.New("workspace role does not allow this action"))
}

func RequireWorkspaceAdmin(principal Principal) error {
	return requireWorkspaceRole(principal, RoleOwner, RoleAdmin)
}

func normalizeInvitableRole(role string) (string, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if role != RoleAdmin && role != RoleViewer {
		return "", errors.Join(domain.ErrInvalidInput, errors.New("role must be admin or viewer"))
	}
	return role, nil
}

func normalizeMemberRole(role string) (string, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if role != RoleOwner && role != RoleAdmin && role != RoleViewer {
		return "", errors.Join(domain.ErrInvalidInput, errors.New("role must be owner, admin, or viewer"))
	}
	return role, nil
}

func invitationStatus(invitation WorkspaceInvitation, now time.Time) string {
	switch {
	case invitation.AcceptedAt != nil:
		return "accepted"
	case invitation.RevokedAt != nil:
		return "revoked"
	case !invitation.ExpiresAt.After(now):
		return "expired"
	default:
		return "pending"
	}
}
