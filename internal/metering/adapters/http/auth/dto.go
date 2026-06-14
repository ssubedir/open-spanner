package auth

type CreateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

type LoginResponse struct {
	Token     string       `json:"token"`
	TokenType string       `json:"token_type"`
	ExpiresAt string       `json:"expires_at"`
	User      UserResponse `json:"user"`
}
