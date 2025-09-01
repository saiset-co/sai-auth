package models

type Role struct {
	InternalID  string                 `json:"internal_id" bson:"internal_id"`
	Name        string                 `json:"name" bson:"name" validate:"required"`
	IsActive    bool                   `json:"is_active" bson:"is_active"`
	ParentRoles []string               `json:"parent_roles" bson:"parent_roles"`
	Permissions []Permission           `json:"permissions" bson:"permissions"`
	Data        map[string]interface{} `json:"data" bson:"data"`
	CrTime      int64                  `json:"cr_time" bson:"cr_time"`
	ChTime      int64                  `json:"ch_time" bson:"ch_time"`
}

type CreateRoleRequest struct {
	Name        string                 `json:"name" validate:"required"`
	IsActive    *bool                  `json:"is_active"`
	ParentRoles []string               `json:"parent_roles"`
	Permissions []Permission           `json:"permissions"`
	Data        map[string]interface{} `json:"data"`
}

type RolePermissionsResponse struct {
	Role        RoleInfo             `json:"role"`
	Users       []string             `json:"users"`
	Permissions []CompiledPermission `json:"permissions"`
}

type RoleInfo struct {
	InternalID string `json:"internal_id"`
	Name       string `json:"name"`
}
