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

type RotateAPIKeyRequest struct {
	GracePeriodSeconds int `json:"grace_period_seconds"`
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

type OAuthProviderResponse struct {
	Enabled bool   `json:"enabled"`
	ID      string `json:"id"`
	Name    string `json:"name"`
}

type OAuthProviderListResponse struct {
	Items               []OAuthProviderResponse `json:"items"`
	RegistrationEnabled bool                    `json:"registration_enabled"`
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
	Status        string   `json:"status"`
}

type APIKeyCreateResponse struct {
	APIKeyResponse
	Key string `json:"key"`
}

type APIKeyListResponse struct {
	Items []APIKeyResponse `json:"items"`
}

type APIKeyEventResponse struct {
	ID              string  `json:"id"`
	APIKeyID        string  `json:"api_key_id"`
	KeyName         string  `json:"key_name"`
	KeyPrefix       string  `json:"key_prefix"`
	EventType       string  `json:"event_type"`
	RelatedAPIKeyID string  `json:"related_api_key_id,omitempty"`
	EffectiveAt     *string `json:"effective_at,omitempty"`
	CreatedAt       string  `json:"created_at"`
}

type APIKeyEventListResponse struct {
	Items []APIKeyEventResponse `json:"items"`
}
