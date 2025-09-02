package models

type Token struct {
	InternalID          string               `json:"internal_id" redis:"internal_id"`
	UserID              string               `json:"user_id" redis:"user_id"`
	AccessToken         string               `json:"access_token" redis:"access_token"`
	RefreshToken        string               `json:"refresh_token" redis:"refresh_token"`
	ExpiresAt           int64                `json:"expires_at" redis:"expires_at"`
	RefreshExpiresAt    int64                `json:"refresh_expires_at" redis:"refresh_expires_at"`
	CompiledPermissions []CompiledPermission `json:"compiled_permissions" redis:"compiled_permissions"`
	CreatedAt           int64                `json:"cr_time" redis:"cr_time"`
	UpdatedAt           int64                `json:"ch_time" redis:"ch_time"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type AuthResponse struct {
	User        *User                `json:"user"`
	Tokens      *TokenResponse       `json:"tokens"`
	Permissions []CompiledPermission `json:"permissions"`
}

type VerifyRequest struct {
	Token         string                 `json:"token" validate:"required"`
	Microservice  string                 `json:"microservice" validate:"required"`
	Method        string                 `json:"method" validate:"required"`
	Path          string                 `json:"path" validate:"required"`
	RequestParams map[string]interface{} `json:"request_params"`
}

type VerifyResponse struct {
	Allowed        bool                   `json:"allowed"`
	UserID         string                 `json:"user_id"`
	ModifiedParams map[string]interface{} `json:"modified_params,omitempty"`
	Reason         string                 `json:"reason,omitempty"`
	ViolatedRule   *ViolatedRule          `json:"violated_restriction,omitempty"`
}

type ViolatedRule struct {
	Param          string `json:"param"`
	AttemptedValue string `json:"attempted_value"`
	RuleType       string `json:"rule_type"`
}

type TestPermissionsRequest struct {
	UserID       string                 `json:"user_id" validate:"required"`
	Microservice string                 `json:"microservice" validate:"required"`
	Method       string                 `json:"method" validate:"required"`
	Path         string                 `json:"path" validate:"required"`
	TestParams   map[string]interface{} `json:"test_params"`
}

type UserInfoResponse struct {
	User        *User                `json:"user"`
	Permissions []CompiledPermission `json:"permissions"`
}
