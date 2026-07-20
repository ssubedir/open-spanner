package auth

import "github.com/go-chi/chi/v5"

func (h *Handler) RegisterRoutes(router chi.Router) {
	h.RegisterPublicRoutes(router)
	router.Group(func(session chi.Router) {
		session.Use(h.RequireSession)
		h.RegisterSessionRoutes(session)
	})
}

func (h *Handler) RegisterPublicRoutes(router chi.Router) {
	router.Get("/auth/workspace-invitations/{token}", h.PreviewWorkspaceInvitation)
	router.Get("/auth/oauth/{provider}", h.StartOAuth)
	router.Get("/auth/oauth/{provider}/callback", h.CompleteOAuth)
	router.Get("/auth/providers", h.ListOAuthProviders)
	router.Post("/auth/users", h.CreateUser)
	router.Delete("/auth/session", h.DeleteSession)
	router.Post("/auth/session/refresh", h.RefreshSession)
	router.Post("/auth/sessions", h.CreateSession)
}

func (h *Handler) RegisterSessionRoutes(router chi.Router) {
	router.Get("/auth/workspaces", h.ListWorkspaces)
	router.Post("/auth/session/workspace", h.SwitchWorkspace)
	router.Get("/auth/workspace/members", h.ListWorkspaceMembers)
	router.Patch("/auth/workspace/members/{user_id}", h.UpdateWorkspaceMember)
	router.Delete("/auth/workspace/members/{user_id}", h.DeleteWorkspaceMember)
	router.Get("/auth/workspace/invitations", h.ListWorkspaceInvitations)
	router.Post("/auth/workspace/invitations", h.CreateWorkspaceInvitation)
	router.Delete("/auth/workspace/invitations/{id}", h.DeleteWorkspaceInvitation)
	router.Post("/auth/workspace-invitations/{token}/accept", h.AcceptWorkspaceInvitation)
	router.Get("/auth/api-keys", h.ListAPIKeys)
	router.Get("/auth/api-key-events", h.ListAPIKeyEvents)
	router.Post("/auth/api-keys", h.CreateAPIKey)
	router.Post("/auth/api-keys/{id}/rotate", h.RotateAPIKey)
	router.Delete("/auth/api-keys/{id}", h.DeleteAPIKey)
	router.Get("/auth/session", h.GetSession)
}
