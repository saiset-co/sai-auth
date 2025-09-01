package models

type User struct {
	InternalID   string                 `json:"internal_id" bson:"internal_id"`
	Username     string                 `json:"username" bson:"username" validate:"required"`
	Email        string                 `json:"email" bson:"email" validate:"required,email"`
	PasswordHash string                 `json:"password_hash,omitempty" bson:"password_hash"`
	IsActive     bool                   `json:"is_active" bson:"is_active"`
	IsSuperUser  bool                   `json:"is_super_user,omitempty" bson:"is_super_user"`
	Roles        []string               `json:"roles" bson:"roles"`
	Data         map[string]interface{} `json:"data" bson:"data"`
	CrTime       int64                  `json:"cr_time,omitempty" bson:"cr_time"`
	ChTime       int64                  `json:"ch_time,omitempty" bson:"ch_time"`
}

type CreateUserRequest struct {
	Username string                 `json:"username" validate:"required"`
	Email    string                 `json:"email" validate:"required,email"`
	Password string                 `json:"password" validate:"required,min=8"`
	IsActive *bool                  `json:"is_active"`
	Data     map[string]interface{} `json:"data"`
}

type LoginRequest struct {
	User     string `json:"user" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}
