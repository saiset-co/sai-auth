package service

import (
	"fmt"
	"github.com/saiset-co/sai-auth/internal/models"
	"github.com/saiset-co/sai-auth/internal/repository"
	saiTypes "github.com/saiset-co/sai-service/types"
	"strings"
)

type PermissionService struct {
	roleRepo repository.RoleRepository
}

func NewPermissionService(roleRepo repository.RoleRepository) *PermissionService {
	return &PermissionService{
		roleRepo: roleRepo,
	}
}

func (s *PermissionService) CompilePermissions(ctx *saiTypes.RequestCtx, user *models.User) ([]models.CompiledPermission, error) {
	if len(user.Roles) == 0 {
		return []models.CompiledPermission{}, nil
	}

	allRoles, err := s.collectAllRoles(ctx, user.Roles, make(map[string]bool), 0)
	if err != nil {
		return nil, err
	}

	permissionMap := make(map[string]*models.CompiledPermission)

	for _, role := range allRoles {
		for _, permission := range role.Permissions {
			key := fmt.Sprintf("%s:%s:%s", permission.Microservice, permission.Method, permission.Path)

			if existing, exists := permissionMap[key]; exists {
				s.mergePermissions(existing, &permission, role.InternalID, user)
			} else {
				compiled := s.compilePermission(&permission, role.InternalID, user)
				permissionMap[key] = compiled
			}
		}
	}

	result := make([]models.CompiledPermission, 0, len(permissionMap))
	for _, permission := range permissionMap {
		result = append(result, *permission)
	}

	return result, nil
}

func (s *PermissionService) collectAllRoles(ctx *saiTypes.RequestCtx, roleIDs []string, visited map[string]bool, depth int) ([]*models.Role, error) {
	if depth > 5 {
		return nil, fmt.Errorf("maximum role inheritance depth exceeded")
	}

	var allRoles []*models.Role
	var parentRoleIDs []string

	for _, roleID := range roleIDs {
		if visited[roleID] {
			continue
		}
		visited[roleID] = true

		role, err := s.roleRepo.GetByID(ctx, roleID)
		if err != nil {
			continue
		}

		if !role.IsActive {
			continue
		}

		allRoles = append(allRoles, role)
		parentRoleIDs = append(parentRoleIDs, role.ParentRoles...)
	}

	if len(parentRoleIDs) > 0 {
		parentRoles, err := s.collectAllRoles(ctx, parentRoleIDs, visited, depth+1)
		if err != nil {
			return nil, err
		}
		allRoles = append(parentRoles, allRoles...)
	}

	return allRoles, nil
}

func (s *PermissionService) compilePermission(permission *models.Permission, roleID string, user *models.User) *models.CompiledPermission {
	compiled := &models.CompiledPermission{
		Microservice:     permission.Microservice,
		Method:           permission.Method,
		Path:             permission.Path,
		Rates:            permission.Rates,
		RequiredParams:   make([]models.Params, 0, len(permission.RequiredParams)),
		RestrictedParams: make([]models.Params, 0, len(permission.RestrictedParams)),
		InheritedFrom:    []string{roleID},
	}

	for _, param := range permission.RequiredParams {
		processedParam := s.processPlaceholders(param, user)
		compiled.RequiredParams = append(compiled.RequiredParams, processedParam)
	}

	for _, param := range permission.RestrictedParams {
		processedParam := s.processPlaceholders(param, user)
		compiled.RestrictedParams = append(compiled.RestrictedParams, processedParam)
	}

	return compiled
}

func (s *PermissionService) mergePermissions(existing *models.CompiledPermission, newPerm *models.Permission, roleID string, user *models.User) {
	existing.InheritedFrom = append(existing.InheritedFrom, roleID)

	existing.Rates = append(existing.Rates, newPerm.Rates...)

	for _, param := range newPerm.RequiredParams {
		processedParam := s.processPlaceholders(param, user)
		found := false
		for i, existingParam := range existing.RequiredParams {
			if existingParam.Param == param.Param {
				existing.RequiredParams[i] = s.mergeParams(existingParam, processedParam)
				found = true
				break
			}
		}
		if !found {
			existing.RequiredParams = append(existing.RequiredParams, processedParam)
		}
	}

	for _, param := range newPerm.RestrictedParams {
		processedParam := s.processPlaceholders(param, user)
		found := false
		for i, existingParam := range existing.RestrictedParams {
			if existingParam.Param == param.Param {
				existing.RestrictedParams[i] = s.mergeParams(existingParam, processedParam)
				found = true
				break
			}
		}
		if !found {
			existing.RestrictedParams = append(existing.RestrictedParams, processedParam)
		}
	}
}

func (s *PermissionService) mergeParams(existing, new models.Params) models.Params {
	result := existing

	if new.Value != "" {
		if existing.Value == "" || existing.Value == "*" {
			result.Value = new.Value
		}
	}

	if len(new.AnyValue) > 0 {
		if len(existing.AnyValue) == 0 {
			result.AnyValue = new.AnyValue
		} else {
			valueMap := make(map[string]bool)
			for _, v := range existing.AnyValue {
				valueMap[v] = true
			}
			for _, v := range new.AnyValue {
				valueMap[v] = true
			}

			result.AnyValue = make([]string, 0, len(valueMap))
			for v := range valueMap {
				result.AnyValue = append(result.AnyValue, v)
			}
		}
	}

	if len(new.AllValues) > 0 {
		if len(existing.AllValues) == 0 {
			result.AllValues = new.AllValues
		} else {
			valueMap := make(map[string]bool)
			for _, v := range existing.AllValues {
				valueMap[v] = true
			}
			for _, v := range new.AllValues {
				valueMap[v] = true
			}

			result.AllValues = make([]string, 0, len(valueMap))
			for v := range valueMap {
				result.AllValues = append(result.AllValues, v)
			}
		}
	}

	return result
}

func (s *PermissionService) processPlaceholders(param models.Params, user *models.User) models.Params {
	result := param

	if param.Value != "" && strings.HasPrefix(param.Value, "$.") {
		result.Value = s.resolvePlaceholder(param.Value, user)
	}

	if len(param.AnyValue) > 0 {
		for i, value := range param.AnyValue {
			if strings.HasPrefix(value, "$.") {
				resolved := s.resolvePlaceholder(value, user)
				if strings.Contains(resolved, ",") {
					values := strings.Split(resolved, ",")
					result.AnyValue = append(result.AnyValue[:i], append(values, result.AnyValue[i+1:]...)...)
				} else {
					result.AnyValue[i] = resolved
				}
			}
		}
	}

	if len(param.AllValues) > 0 {
		for i, value := range param.AllValues {
			if strings.HasPrefix(value, "$.") {
				resolved := s.resolvePlaceholder(value, user)
				if strings.Contains(resolved, ",") {
					values := strings.Split(resolved, ",")
					result.AllValues = append(result.AllValues[:i], append(values, result.AllValues[i+1:]...)...)
				} else {
					result.AllValues[i] = resolved
				}
			}
		}
	}

	return result
}

func (s *PermissionService) resolvePlaceholder(placeholder string, user *models.User) string {
	path := strings.TrimPrefix(placeholder, "$.")
	parts := strings.Split(path, ".")

	if len(parts) == 1 && parts[0] == "internal_id" {
		return user.InternalID
	}

	if len(parts) >= 2 && parts[0] == "data" {
		if user.Data == nil {
			return ""
		}

		current := user.Data
		for i := 1; i < len(parts); i++ {
			if value, exists := current[parts[i]]; exists {
				switch v := value.(type) {
				case string:
					return v
				case []interface{}:
					strValues := make([]string, len(v))
					for j, item := range v {
						strValues[j] = fmt.Sprintf("%v", item)
					}
					return strings.Join(strValues, ",")
				case []string:
					return strings.Join(v, ",")
				default:
					return fmt.Sprintf("%v", v)
				}
			}
			break
		}
	}

	return ""
}

func (s *PermissionService) CheckPermission(ctx *saiTypes.RequestCtx, permissions []models.CompiledPermission, microservice, method, path string, requestParams map[string]interface{}) (*models.VerifyResponse, error) {
	var matchedPermission *models.CompiledPermission

	for i := range permissions {
		perm := &permissions[i]
		if perm.Microservice == microservice && perm.Method == method && s.matchPath(perm.Path, path) {
			matchedPermission = perm
			break
		}
	}

	if matchedPermission == nil {
		return &models.VerifyResponse{
			Allowed: false,
			Reason:  fmt.Sprintf("No permission found for %s %s %s", microservice, method, path),
		}, nil
	}

	for _, restriction := range matchedPermission.RestrictedParams {
		if value, exists := requestParams[restriction.Param]; exists {
			if s.isRestricted(value, restriction) {
				return &models.VerifyResponse{
					Allowed: false,
					Reason:  fmt.Sprintf("Access denied to %s '%v'", restriction.Param, value),
					ViolatedRule: &models.ViolatedRule{
						Param:          restriction.Param,
						AttemptedValue: fmt.Sprintf("%v", value),
						RuleType:       "restricted_params",
					},
				}, nil
			}
		}
	}

	modifiedParams := make(map[string]interface{})
	for key, value := range requestParams {
		modifiedParams[key] = value
	}

	for _, requirement := range matchedPermission.RequiredParams {
		if value, exists := requestParams[requirement.Param]; exists {
			if !s.satisfiesRequirement(value, requirement) {
				return &models.VerifyResponse{
					Allowed: false,
					Reason:  fmt.Sprintf("Parameter %s does not satisfy requirements", requirement.Param),
					ViolatedRule: &models.ViolatedRule{
						Param:          requirement.Param,
						AttemptedValue: fmt.Sprintf("%v", value),
						RuleType:       "required_params",
					},
				}, nil
			}
		} else {
			if requirement.Value != "" && requirement.Value != "*" {
				modifiedParams[requirement.Param] = requirement.Value
			}
		}
	}

	return &models.VerifyResponse{
		Allowed:        true,
		ModifiedParams: modifiedParams,
	}, nil
}

func (s *PermissionService) isRestricted(value interface{}, restriction models.Params) bool {
	valueStr := fmt.Sprintf("%v", value)

	if restriction.Value != "" {
		return valueStr == restriction.Value
	}

	if len(restriction.AnyValue) > 0 {
		for _, restrictedValue := range restriction.AnyValue {
			if valueStr == restrictedValue {
				return true
			}
		}
	}

	if len(restriction.AllValues) > 0 {
		if valueSlice, ok := value.([]interface{}); ok {
			restrictedMap := make(map[string]bool)
			for _, rv := range restriction.AllValues {
				restrictedMap[rv] = true
			}

			for _, v := range valueSlice {
				if restrictedMap[fmt.Sprintf("%v", v)] {
					return true
				}
			}
		}
	}

	return false
}

func (s *PermissionService) satisfiesRequirement(value interface{}, requirement models.Params) bool {
	if requirement.Value == "*" {
		return true
	}

	if requirement.Value != "" {
		return fmt.Sprintf("%v", value) == requirement.Value
	}

	if len(requirement.AnyValue) > 0 {
		valueStr := fmt.Sprintf("%v", value)
		for _, allowedValue := range requirement.AnyValue {
			if valueStr == allowedValue {
				return true
			}
		}
		return false
	}

	if len(requirement.AllValues) > 0 {
		if valueSlice, ok := value.([]interface{}); ok {
			requiredMap := make(map[string]bool)
			for _, rv := range requirement.AllValues {
				requiredMap[rv] = true
			}

			for _, rv := range requirement.AllValues {
				found := false
				for _, v := range valueSlice {
					if fmt.Sprintf("%v", v) == rv {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}
			return true
		}
		return false
	}

	return true
}

func (s *PermissionService) matchPath(permissionPath, requestPath string) bool {
	// Exact match
	if permissionPath == requestPath {
		return true
	}
	
	// Wildcard match - /api/v1/* matches /api/v1/, /api/v1/documents, etc.
	if strings.HasSuffix(permissionPath, "/*") {
		prefix := strings.TrimSuffix(permissionPath, "/*")
		return strings.HasPrefix(requestPath, prefix)
	}
	
	// Wildcard match - /api/v1* matches /api/v1, /api/v1/, /api/v1/documents, etc.
	if strings.HasSuffix(permissionPath, "*") {
		prefix := strings.TrimSuffix(permissionPath, "*")
		return strings.HasPrefix(requestPath, prefix)
	}
	
	return false
}
