package types

type PaginationRequest struct {
	Page   int    `json:"page" form:"page"`
	Limit  int    `json:"limit" form:"limit"`
	Search string `json:"search" form:"search"`
}

type UserFilterRequest struct {
	PaginationRequest
	Role   string `json:"role" form:"role"`
	Active *bool  `json:"active" form:"active"`
}

type RoleFilterRequest struct {
	PaginationRequest
	Active *bool `json:"active" form:"active"`
}

type TokenFilterRequest struct {
	PaginationRequest
	UserID string `json:"user_id" form:"user_id"`
	Active *bool  `json:"active" form:"active"`
}

type UpdateRequest struct {
	Filter map[string]interface{} `json:"filter" validate:"required"`
	Data   map[string]interface{} `json:"data" validate:"required"`
}

type DeleteRequest struct {
	Filter map[string]interface{} `json:"filter" validate:"required"`
}

type GetRequest struct {
	Filter map[string]interface{} `json:"filter"`
	Sort   map[string]interface{} `json:"sort"`
	Limit  int                    `json:"limit"`
	Skip   int                    `json:"skip"`
}
