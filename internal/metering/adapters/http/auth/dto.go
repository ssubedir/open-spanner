package auth

type CreateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CreateAPIKeyRequest struct {
	Name          string   `json:"name"`
	Scopes        []string `json:"scopes,omitempty"`
	AllowedMeters []string `json:"allowed_meters,omitempty"`
	ExpiresAt     string   `json:"expires_at,omitempty"`
}

type UserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

type LoginResponse struct {
	ExpiresAt string       `json:"expires_at"`
	User      UserResponse `json:"user"`
}

type RefreshResponse struct {
	ExpiresAt string       `json:"expires_at"`
	User      UserResponse `json:"user"`
}

type SessionResponse struct {
	User UserResponse `json:"user"`
}

type APIKeyResponse struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Prefix        string   `json:"prefix"`
	Scopes        []string `json:"scopes"`
	AllowedMeters []string `json:"allowed_meters"`
	ExpiresAt     *string  `json:"expires_at,omitempty"`
	RevokedAt     *string  `json:"revoked_at,omitempty"`
	CreatedAt     string   `json:"created_at"`
	LastUsedAt    *string  `json:"last_used_at,omitempty"`
}

type APIKeyCreateResponse struct {
	APIKeyResponse
	Key string `json:"key"`
}

type APIKeyListResponse struct {
	Items []APIKeyResponse `json:"items"`
}
