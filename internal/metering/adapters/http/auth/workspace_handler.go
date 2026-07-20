package auth

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	appauth "github.com/ssubedir/open-spanner/internal/auth"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/request"
	"github.com/ssubedir/open-spanner/internal/metering/adapters/http/internal/respond"
)

// ListWorkspaces lists workspaces available to the current user.
// @Summary List workspaces
// @ID listWorkspaces
// @Tags workspaces
// @Produce json
// @Success 200 {object} WorkspaceListResponse
// @Router /v1/auth/workspaces [get]
func (h *Handler) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	principal, err := h.currentPrincipal(r)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	accesses, err := h.service.ListWorkspaces(r.Context(), principal.User.ID)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	items := make([]WorkspaceResponse, 0, len(accesses))
	for _, access := range accesses {
		items = append(items, WorkspaceResponse{ID: access.Workspace.ID, Name: access.Workspace.Name, Role: access.Membership.Role, CreatedAt: access.Workspace.CreatedAt.Format(time.RFC3339), JoinedAt: access.Membership.CreatedAt.Format(time.RFC3339)})
	}
	respond.JSON(w, http.StatusOK, WorkspaceListResponse{Items: items})
}

// SwitchWorkspace issues a new session scoped to another workspace.
// @Summary Switch workspace
// @ID switchWorkspace
// @Tags workspaces
// @Accept json
// @Produce json
// @Param request body SwitchWorkspaceRequest true "Workspace"
// @Success 200 {object} LoginResponse
// @Router /v1/auth/session/workspace [post]
func (h *Handler) SwitchWorkspace(w http.ResponseWriter, r *http.Request) {
	principal, err := h.currentPrincipal(r)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	var req SwitchWorkspaceRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}
	session, err := h.service.SwitchWorkspace(r.Context(), principal.User.ID, req.WorkspaceID)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	h.replaceSessionCookies(w, r, session)
}

// ListWorkspaceMembers lists members of the current workspace.
// @Summary List workspace members
// @ID listWorkspaceMembers
// @Tags workspaces
// @Produce json
// @Success 200 {object} WorkspaceMemberListResponse
// @Router /v1/auth/workspace/members [get]
func (h *Handler) ListWorkspaceMembers(w http.ResponseWriter, r *http.Request) {
	principal, err := h.currentPrincipal(r)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	members, err := h.service.ListWorkspaceMembers(r.Context(), principal)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	items := make([]WorkspaceMemberResponse, 0, len(members))
	for _, member := range members {
		items = append(items, WorkspaceMemberResponse{UserID: member.UserID, Email: member.Email, Role: member.Role, CreatedAt: member.CreatedAt.Format(time.RFC3339)})
	}
	respond.JSON(w, http.StatusOK, WorkspaceMemberListResponse{Items: items})
}

// UpdateWorkspaceMember changes a workspace member role.
// @Summary Update workspace member
// @ID updateWorkspaceMember
// @Tags workspaces
// @Accept json
// @Param user_id path string true "User ID"
// @Param request body UpdateWorkspaceMemberRequest true "Role"
// @Success 204
// @Router /v1/auth/workspace/members/{user_id} [patch]
func (h *Handler) UpdateWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	principal, err := h.currentPrincipal(r)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	var req UpdateWorkspaceMemberRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}
	err = h.service.UpdateWorkspaceMember(r.Context(), principal, appauth.UpdateWorkspaceMemberCommand{UserID: chi.URLParam(r, "user_id"), Role: req.Role})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteWorkspaceMember removes a workspace member.
// @Summary Remove workspace member
// @ID deleteWorkspaceMember
// @Tags workspaces
// @Param user_id path string true "User ID"
// @Success 204
// @Router /v1/auth/workspace/members/{user_id} [delete]
func (h *Handler) DeleteWorkspaceMember(w http.ResponseWriter, r *http.Request) {
	principal, err := h.currentPrincipal(r)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	if err := h.service.RemoveWorkspaceMember(r.Context(), principal, chi.URLParam(r, "user_id")); err != nil {
		respond.ServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListWorkspaceInvitations lists invitation history for the current workspace.
// @Summary List workspace invitations
// @ID listWorkspaceInvitations
// @Tags workspaces
// @Produce json
// @Success 200 {object} WorkspaceInvitationListResponse
// @Router /v1/auth/workspace/invitations [get]
func (h *Handler) ListWorkspaceInvitations(w http.ResponseWriter, r *http.Request) {
	principal, err := h.currentPrincipal(r)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	invitations, err := h.service.ListWorkspaceInvitations(r.Context(), principal)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	items := make([]WorkspaceInvitationResponse, 0, len(invitations))
	for _, invitation := range invitations {
		items = append(items, workspaceInvitationResponse(invitation))
	}
	respond.JSON(w, http.StatusOK, WorkspaceInvitationListResponse{Items: items})
}

// CreateWorkspaceInvitation creates a seven-day single-use invitation.
// @Summary Create workspace invitation
// @ID createWorkspaceInvitation
// @Tags workspaces
// @Accept json
// @Produce json
// @Param request body CreateWorkspaceInvitationRequest true "Invitation"
// @Success 201 {object} WorkspaceInvitationResponse
// @Router /v1/auth/workspace/invitations [post]
func (h *Handler) CreateWorkspaceInvitation(w http.ResponseWriter, r *http.Request) {
	principal, err := h.currentPrincipal(r)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	var req CreateWorkspaceInvitationRequest
	if err := request.DecodeJSON(r.Body, &req); err != nil {
		respond.ValidationError(w, err)
		return
	}
	invitation, err := h.service.CreateWorkspaceInvitation(r.Context(), principal, appauth.CreateWorkspaceInvitationCommand{Email: req.Email, Role: req.Role})
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, workspaceInvitationResponse(invitation))
}

// PreviewWorkspaceInvitation returns public invitation metadata.
// @Summary Preview workspace invitation
// @ID previewWorkspaceInvitation
// @Tags workspaces
// @Produce json
// @Param token path string true "Invitation token"
// @Success 200 {object} WorkspaceInvitationResponse
// @Router /v1/auth/workspace-invitations/{token} [get]
func (h *Handler) PreviewWorkspaceInvitation(w http.ResponseWriter, r *http.Request) {
	invitation, err := h.service.PreviewWorkspaceInvitation(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, workspaceInvitationResponse(invitation))
}

// AcceptWorkspaceInvitation joins the invited workspace and switches session scope.
// @Summary Accept workspace invitation
// @ID acceptWorkspaceInvitation
// @Tags workspaces
// @Produce json
// @Param token path string true "Invitation token"
// @Success 200 {object} LoginResponse
// @Router /v1/auth/workspace-invitations/{token}/accept [post]
func (h *Handler) AcceptWorkspaceInvitation(w http.ResponseWriter, r *http.Request) {
	principal, err := h.currentPrincipal(r)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	session, err := h.service.AcceptWorkspaceInvitation(r.Context(), principal, chi.URLParam(r, "token"))
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	h.replaceSessionCookies(w, r, session)
}

// DeleteWorkspaceInvitation revokes an active invitation.
// @Summary Revoke workspace invitation
// @ID deleteWorkspaceInvitation
// @Tags workspaces
// @Param id path string true "Invitation ID"
// @Success 204
// @Router /v1/auth/workspace/invitations/{id} [delete]
func (h *Handler) DeleteWorkspaceInvitation(w http.ResponseWriter, r *http.Request) {
	principal, err := h.currentPrincipal(r)
	if err != nil {
		respond.ServiceError(w, err)
		return
	}
	if err := h.service.RevokeWorkspaceInvitation(r.Context(), principal, chi.URLParam(r, "id")); err != nil {
		respond.ServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) replaceSessionCookies(w http.ResponseWriter, r *http.Request, session appauth.LoginResult) {
	setAuthCookie(w, r, accessCookieName, session.AccessToken, session.AccessExpiresAt)
	setAuthCookie(w, r, refreshCookieName, session.RefreshToken, session.RefreshExpiresAt)
	respond.JSON(w, http.StatusOK, LoginResponse{ExpiresAt: session.AccessExpiresAt.Format(time.RFC3339), User: userResponse(session.User)})
}

func workspaceInvitationResponse(invitation appauth.WorkspaceInvitationResult) WorkspaceInvitationResponse {
	return WorkspaceInvitationResponse{ID: invitation.ID, WorkspaceID: invitation.WorkspaceID, WorkspaceName: invitation.WorkspaceName, Email: invitation.Email, Role: invitation.Role, Status: invitation.Status, ExpiresAt: invitation.ExpiresAt.Format(time.RFC3339), CreatedAt: invitation.CreatedAt.Format(time.RFC3339), Token: invitation.Token}
}
