package internal

import (
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/saiset-co/sai-auth/internal/models"
	"github.com/saiset-co/sai-auth/internal/repository"
	"github.com/saiset-co/sai-auth/internal/service"
	"github.com/saiset-co/sai-auth/types"
	"github.com/saiset-co/sai-service/admin"
	"github.com/saiset-co/sai-service/sai"
	saiTypes "github.com/saiset-co/sai-service/types"
)

type AdminPanel struct {
	userSvc   *service.UserService
	roleSvc   *service.RoleService
	tokenRepo repository.TokenRepository
}

func SetupAdmin(userSvc *service.UserService, roleSvc *service.RoleService, tokenRepo repository.TokenRepository) {
	sai.InstallLogBuffer()

	panel := &AdminPanel{
		userSvc:   userSvc,
		roleSvc:   roleSvc,
		tokenRepo: tokenRepo,
	}

	adminGroup := sai.Router().Group("/admin").
		WithAuthProvider("basic").
		WithTimeout(30 * time.Second)

	adminGroup.POST("/users/create", panel.handleCreateUser)
	adminGroup.POST("/users/update", panel.handleUpdateUser)
	adminGroup.POST("/users/delete", panel.handleDeleteUser)
	adminGroup.POST("/roles/create", panel.handleCreateRole)
	adminGroup.POST("/roles/update", panel.handleUpdateRole)
	adminGroup.POST("/roles/delete", panel.handleDeleteRole)
	adminGroup.POST("/tokens/delete", panel.handleDeleteToken)
	adminGroup.GET("/ajax/service-logs", panel.handleAjaxServiceLogs)

	sai.Admin(adminGroup).
		WithTitle("SAI Auth Admin").
		WithSubtitle("Управление пользователями, ролями и токенами авторизации.").
		WithAuthProvider("basic").
		WithHomePage("Дашборд", "Статистика и активные токены.", func(ctx *saiTypes.RequestCtx) (*admin.PageData, error) {
			return panel.buildDashboardPage(ctx)
		}).
		Page("roles", "Роли", func(ctx *saiTypes.RequestCtx) (*admin.PageData, error) {
			return panel.buildRolesPage(ctx)
		}).
		Page("users", "Пользователи", func(ctx *saiTypes.RequestCtx) (*admin.PageData, error) {
			return panel.buildUsersPage(ctx)
		}).
		Group("Логи").
		Page("service-logs", "Сервис", panel.pageServiceLogs).
		Mount()
}

func (p *AdminPanel) buildDashboardPage(ctx *saiTypes.RequestCtx) (*admin.PageData, error) {
	_, userTotal, err := p.userSvc.List(ctx, &types.UserFilterRequest{PaginationRequest: types.PaginationRequest{Limit: 1}})
	if err != nil {
		return nil, err
	}

	_, roleTotal, err := p.roleSvc.List(ctx, &types.RoleFilterRequest{PaginationRequest: types.PaginationRequest{Limit: 1}})
	if err != nil {
		return nil, err
	}

	tokens, tokenTotal, _ := p.tokenRepo.List(ctx, &types.TokenFilterRequest{PaginationRequest: types.PaginationRequest{Limit: 50}})

	activeTokens := 0
	now := time.Now().UnixNano()
	for _, t := range tokens {
		if t.ExpiresAt > now {
			activeTokens++
		}
	}

	return &admin.PageData{
		Notices: admin.ReadFlash(ctx, "/admin"),
		Stats: []admin.Stat{
			{Label: "Пользователи", Value: int(userTotal), Tone: "ok"},
			{Label: "Роли", Value: int(roleTotal)},
			{Label: "Токены всего", Value: int(tokenTotal)},
			{Label: "Токены активные", Value: activeTokens, Tone: admin.ToneFromCount(activeTokens, "success", "ok")},
		},
		Sections: []admin.Section{
			{
				Title:       "Токены (последние 50)",
				ContentHTML: renderTokensTable(tokens),
			},
		},
	}, nil
}

func (p *AdminPanel) buildRolesPage(ctx *saiTypes.RequestCtx) (*admin.PageData, error) {
	roles, _, err := p.roleSvc.List(ctx, &types.RoleFilterRequest{PaginationRequest: types.PaginationRequest{Limit: 100}})
	if err != nil {
		return nil, err
	}

	sections := []admin.Section{}

	editID := strings.TrimSpace(string(ctx.QueryArgs().Peek("edit")))
	if editID != "" {
		role, err := p.roleSvc.GetByID(ctx, editID)
		if err == nil {
			sections = append(sections, admin.Section{
				Title:       "Редактировать роль",
				ContentHTML: renderEditRoleForm(role, roles),
			})
		}
	} else {
		sections = append(sections, admin.Section{
			Title:       "Создать роль",
			ContentHTML: renderCreateRoleForm(roles),
		})
	}
	sections = append(sections, admin.Section{
		Title:       "Список ролей",
		ContentHTML: renderRolesTable(roles),
	})

	return &admin.PageData{
		Notices:  admin.ReadFlash(ctx, "/admin/pages/roles"),
		Sections: sections,
	}, nil
}

func (p *AdminPanel) buildUsersPage(ctx *saiTypes.RequestCtx) (*admin.PageData, error) {
	users, _, err := p.userSvc.List(ctx, &types.UserFilterRequest{PaginationRequest: types.PaginationRequest{Limit: 100}})
	if err != nil {
		return nil, err
	}

	roles, _, _ := p.roleSvc.List(ctx, &types.RoleFilterRequest{PaginationRequest: types.PaginationRequest{Limit: 100}})

	sections := []admin.Section{}

	editID := strings.TrimSpace(string(ctx.QueryArgs().Peek("edit")))
	if editID != "" {
		user, err := p.userSvc.GetByID(ctx, editID)
		if err == nil {
			sections = append(sections, admin.Section{
				Title:       "Редактировать пользователя",
				ContentHTML: renderEditUserForm(user, roles),
			})
		}
	} else {
		sections = append(sections, admin.Section{
			Title:       "Создать пользователя",
			ContentHTML: renderCreateUserForm(roles),
		})
	}
	sections = append(sections, admin.Section{
		Title:       "Список пользователей",
		ContentHTML: renderUsersTable(users),
	})

	return &admin.PageData{
		Notices:  admin.ReadFlash(ctx, "/admin/pages/users"),
		Sections: sections,
	}, nil
}

func (p *AdminPanel) handleCreateUser(ctx *saiTypes.RequestCtx) {
	isActive := string(ctx.FormValue("is_active")) == "on"

	req := &models.CreateUserRequest{
		Username: strings.TrimSpace(string(ctx.FormValue("username"))),
		Email:    strings.TrimSpace(string(ctx.FormValue("email"))),
		Password: string(ctx.FormValue("password")),
		IsActive: &isActive,
	}

	user, err := p.userSvc.Create(ctx, req)
	if err != nil {
		respondAdmin(ctx, "/admin/pages/users", "", err)
		return
	}

	if roleIDs := parseCSV(string(ctx.FormValue("roles"))); len(roleIDs) > 0 {
		_ = p.userSvc.AssignRoles(ctx, user.InternalID, roleIDs)
	}

	respondAdmin(ctx, "/admin/pages/users", "Пользователь создан", nil)
}

func (p *AdminPanel) handleUpdateUser(ctx *saiTypes.RequestCtx) {
	userID := strings.TrimSpace(string(ctx.FormValue("user_id")))
	if userID == "" {
		respondAdmin(ctx, "/admin/pages/users", "", fmt.Errorf("user_id обязателен"))
		return
	}

	isActive := string(ctx.FormValue("is_active")) == "on"
	data := map[string]any{
		"username":  strings.TrimSpace(string(ctx.FormValue("username"))),
		"email":     strings.TrimSpace(string(ctx.FormValue("email"))),
		"is_active": isActive,
	}

	if pwd := string(ctx.FormValue("password")); strings.TrimSpace(pwd) != "" {
		data["password"] = pwd
	}

	if err := p.userSvc.Update(ctx, map[string]any{"internal_id": userID}, data); err != nil {
		respondAdmin(ctx, "/admin/pages/users", "", err)
		return
	}

	newRoles := parseCSV(string(ctx.FormValue("roles")))
	user, err := p.userSvc.GetByID(ctx, userID)
	if err == nil {
		toRemove := difference(user.Roles, newRoles)
		toAdd := difference(newRoles, user.Roles)
		if len(toRemove) > 0 {
			_ = p.userSvc.RemoveRoles(ctx, userID, toRemove)
		}
		if len(toAdd) > 0 {
			_ = p.userSvc.AssignRoles(ctx, userID, toAdd)
		}
	}

	respondAdmin(ctx, "/admin/pages/users", "Пользователь обновлён", nil)
}

func (p *AdminPanel) handleDeleteUser(ctx *saiTypes.RequestCtx) {
	userID := strings.TrimSpace(string(ctx.FormValue("user_id")))
	if userID == "" {
		respondAdmin(ctx, "/admin/pages/users", "", fmt.Errorf("user_id обязателен"))
		return
	}
	respondAdmin(ctx, "/admin/pages/users", "Пользователь удалён", p.userSvc.Delete(ctx, map[string]any{"internal_id": userID}))
}

func (p *AdminPanel) handleCreateRole(ctx *saiTypes.RequestCtx) {
	isActive := string(ctx.FormValue("is_active")) == "on"
	req := &models.CreateRoleRequest{
		Name:        strings.TrimSpace(string(ctx.FormValue("name"))),
		IsActive:    &isActive,
		ParentRoles: parseCSV(string(ctx.FormValue("parent_roles"))),
		Permissions: parsePermissions(ctx),
	}
	_, err := p.roleSvc.Create(ctx, req)
	respondAdmin(ctx, "/admin/pages/roles", "Роль создана", err)
}

func (p *AdminPanel) handleUpdateRole(ctx *saiTypes.RequestCtx) {
	roleID := strings.TrimSpace(string(ctx.FormValue("role_id")))
	if roleID == "" {
		respondAdmin(ctx, "/admin/pages/roles", "", fmt.Errorf("role_id обязателен"))
		return
	}

	isActive := string(ctx.FormValue("is_active")) == "on"
	data := map[string]any{
		"name":         strings.TrimSpace(string(ctx.FormValue("name"))),
		"is_active":    isActive,
		"parent_roles": parseCSV(string(ctx.FormValue("parent_roles"))),
		"permissions":  parsePermissions(ctx),
	}

	err := p.roleSvc.Update(ctx, map[string]any{"internal_id": roleID}, data)
	respondAdmin(ctx, "/admin/pages/roles", "Роль обновлена", err)
}

func (p *AdminPanel) handleDeleteRole(ctx *saiTypes.RequestCtx) {
	roleID := strings.TrimSpace(string(ctx.FormValue("role_id")))
	if roleID == "" {
		respondAdmin(ctx, "/admin/pages/roles", "", fmt.Errorf("role_id обязателен"))
		return
	}
	respondAdmin(ctx, "/admin/pages/roles", "Роль удалена", p.roleSvc.Delete(ctx, map[string]any{"internal_id": roleID}))
}

func (p *AdminPanel) handleDeleteToken(ctx *saiTypes.RequestCtx) {
	tokenID := strings.TrimSpace(string(ctx.FormValue("token_id")))
	if tokenID == "" {
		respondAdmin(ctx, "/admin", "", fmt.Errorf("token_id обязателен"))
		return
	}
	respondAdmin(ctx, "/admin", "Токен удалён", p.tokenRepo.Delete(ctx, tokenID))
}

func parsePermissions(ctx *saiTypes.RequestCtx) []models.Permission {
	count := countFormArray(ctx, "perm_microservice")
	permissions := make([]models.Permission, 0, count)
	for i := range count {
		ms := strings.TrimSpace(string(ctx.FormValue(fmt.Sprintf("perm_microservice_%d", i))))
		method := strings.TrimSpace(string(ctx.FormValue(fmt.Sprintf("perm_method_%d", i))))
		path := strings.TrimSpace(string(ctx.FormValue(fmt.Sprintf("perm_path_%d", i))))
		if ms == "" || method == "" || path == "" {
			continue
		}
		perm := models.Permission{Microservice: ms, Method: method, Path: path}
		for j := range countFormArray(ctx, fmt.Sprintf("perm_rp_param_%d", i)) {
			param := strings.TrimSpace(string(ctx.FormValue(fmt.Sprintf("perm_rp_param_%d_%d", i, j))))
			value := strings.TrimSpace(string(ctx.FormValue(fmt.Sprintf("perm_rp_value_%d_%d", i, j))))
			if param != "" {
				perm.RequiredParams = append(perm.RequiredParams, models.Params{Param: param, Value: value})
			}
		}
		for j := range countFormArray(ctx, fmt.Sprintf("perm_rsp_param_%d", i)) {
			param := strings.TrimSpace(string(ctx.FormValue(fmt.Sprintf("perm_rsp_param_%d_%d", i, j))))
			value := strings.TrimSpace(string(ctx.FormValue(fmt.Sprintf("perm_rsp_value_%d_%d", i, j))))
			if param != "" {
				perm.RestrictedParams = append(perm.RestrictedParams, models.Params{Param: param, Value: value})
			}
		}
		permissions = append(permissions, perm)
	}
	return permissions
}

func parseCSV(raw string) []string {
	var result []string
	for id := range strings.SplitSeq(strings.TrimSpace(raw), ",") {
		if id = strings.TrimSpace(id); id != "" {
			result = append(result, id)
		}
	}
	return result
}

func difference(a, b []string) []string {
	set := make(map[string]bool, len(b))
	for _, v := range b {
		set[v] = true
	}
	var result []string
	for _, v := range a {
		if !set[v] {
			result = append(result, v)
		}
	}
	return result
}

func respondAdmin(ctx *saiTypes.RequestCtx, redirectPath, message string, err error) {
	if admin.IsActionRequest(ctx) {
		admin.WriteActionJSON(ctx, message, err)
		return
	}
	admin.RedirectWithFlash(ctx, redirectPath, message, err)
}

func countFormArray(ctx *saiTypes.RequestCtx, prefix string) int {
	for i := range 50 {
		if ctx.FormValue(fmt.Sprintf("%s_%d", prefix, i)) == nil {
			return i
		}
	}
	return 50
}

func roleChipsJS(roles []*models.Role, _ string, fnName string) string {
	if len(roles) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<div class="mt-2 flex flex-wrap gap-2">`)
	for _, r := range roles {
		id := template.HTMLEscapeString(r.InternalID)
		name := template.HTMLEscapeString(r.Name)
		b.WriteString(`<button type="button" onclick="` + fnName + `('` + id + `')" title="` + id + `" class="rounded-lg bg-slate-100 px-3 py-1 text-xs font-mono text-slate-600 ring-1 ring-slate-200 hover:bg-brand-100 hover:text-brand-700 hover:ring-brand-300 transition">` + name + `</button>`)
	}
	b.WriteString(`</div>`)
	return b.String()
}

func inputClass() string {
	return `block h-11 w-full rounded-xl border-0 bg-slate-50 px-4 text-sm text-slate-900 ring-1 ring-inset ring-slate-300 placeholder:text-slate-400 focus:ring-2 focus:ring-inset focus:ring-brand-500`
}

func renderCreateUserForm(roles []*models.Role) template.HTML {
	chips := roleChipsJS(roles, "u_roles", "appendRoleID")
	var b strings.Builder
	b.WriteString(`<form method="post" action="/admin/users/create" data-admin-ajax="true" class="grid gap-4">`)
	b.WriteString(`<div class="grid gap-4 lg:grid-cols-3">`)
	b.WriteString(`<div><label class="mb-2 block text-sm font-medium text-slate-700">Логин</label><input name="username" placeholder="username" required class="` + inputClass() + `"></div>`)
	b.WriteString(`<div><label class="mb-2 block text-sm font-medium text-slate-700">Email</label><input name="email" type="email" placeholder="user@example.com" required class="` + inputClass() + `"></div>`)
	b.WriteString(`<div><label class="mb-2 block text-sm font-medium text-slate-700">Пароль (мин. 8 символов)</label><input name="password" type="password" placeholder="••••••••" required minlength="8" class="` + inputClass() + `"></div>`)
	b.WriteString(`</div>`)
	b.WriteString(`<div><label class="mb-2 block text-sm font-medium text-slate-700">Роли</label>`)
	b.WriteString(`<input id="u_roles" name="roles" placeholder="uuid1, uuid2" class="` + inputClass() + ` font-mono">`)
	b.WriteString(`<script>function appendRoleID(id){var el=document.getElementById('u_roles');el.value=el.value.trim()?el.value.trim()+', '+id:id;}</script>`)
	b.WriteString(chips)
	b.WriteString(`</div>`)
	b.WriteString(`<div class="flex items-center gap-4"><label class="flex items-center gap-2 text-sm text-slate-700 cursor-pointer"><input type="checkbox" name="is_active" checked class="rounded"> Активен</label>`)
	b.WriteString(`<button type="submit" class="inline-flex h-11 items-center rounded-xl bg-brand-600 px-5 text-sm font-semibold text-white transition hover:bg-brand-500">Создать</button></div>`)
	b.WriteString(`</form>`)
	return template.HTML(b.String())
}

func renderEditUserForm(user *models.User, roles []*models.Role) template.HTML {
	chips := roleChipsJS(roles, "eu_roles", "appendEditRoleID")
	currentRoles := template.HTMLEscapeString(strings.Join(user.Roles, ", "))
	userID := template.HTMLEscapeString(user.InternalID)
	username := template.HTMLEscapeString(user.Username)
	email := template.HTMLEscapeString(user.Email)
	activeChecked := ""
	if user.IsActive {
		activeChecked = " checked"
	}

	var b strings.Builder
	b.WriteString(`<form method="post" action="/admin/users/update" data-admin-ajax="true" class="grid gap-4">`)
	b.WriteString(`<input type="hidden" name="user_id" value="` + userID + `">`)
	b.WriteString(`<div class="grid gap-4 lg:grid-cols-3">`)
	b.WriteString(`<div><label class="mb-2 block text-sm font-medium text-slate-700">Логин</label><input name="username" value="` + username + `" required class="` + inputClass() + `"></div>`)
	b.WriteString(`<div><label class="mb-2 block text-sm font-medium text-slate-700">Email</label><input name="email" type="email" value="` + email + `" required class="` + inputClass() + `"></div>`)
	b.WriteString(`<div><label class="mb-2 block text-sm font-medium text-slate-700">Новый пароль (оставьте пустым — без изменений)</label><input name="password" type="password" placeholder="••••••••" minlength="8" class="` + inputClass() + `"></div>`)
	b.WriteString(`</div>`)
	b.WriteString(`<div><label class="mb-2 block text-sm font-medium text-slate-700">Роли</label>`)
	b.WriteString(`<input id="eu_roles" name="roles" value="` + currentRoles + `" placeholder="uuid1, uuid2" class="` + inputClass() + ` font-mono">`)
	b.WriteString(`<script>function appendEditRoleID(id){var el=document.getElementById('eu_roles');el.value=el.value.trim()?el.value.trim()+', '+id:id;}</script>`)
	b.WriteString(chips)
	b.WriteString(`</div>`)
	b.WriteString(`<div class="flex items-center gap-4"><label class="flex items-center gap-2 text-sm text-slate-700 cursor-pointer"><input type="checkbox" name="is_active"` + activeChecked + ` class="rounded"> Активен</label>`)
	b.WriteString(`<button type="submit" class="inline-flex h-11 items-center rounded-xl bg-brand-600 px-5 text-sm font-semibold text-white transition hover:bg-brand-500">Сохранить</button>`)
	b.WriteString(`<a href="/admin/pages/users" class="inline-flex h-11 items-center rounded-xl bg-slate-100 px-5 text-sm font-medium text-slate-700 transition hover:bg-slate-200">Отмена</a></div>`)
	b.WriteString(`</form>`)
	return template.HTML(b.String())
}

func renderUsersTable(users []*models.User) template.HTML {
	if len(users) == 0 {
		return template.HTML(`<div class="rounded-2xl border border-dashed border-slate-300 bg-slate-50 px-6 py-10 text-center text-sm text-slate-500">Пользователи не найдены.</div>`)
	}

	var b strings.Builder
	b.WriteString(`<div class="overflow-hidden rounded-[22px] border border-slate-200"><div class="overflow-x-auto"><table class="min-w-full divide-y divide-slate-200">`)
	b.WriteString(`<thead class="bg-slate-50"><tr>`)
	for _, h := range []string{"Логин", "Email", "internal_id", "Ролей", "Активен", "Супер", "Действия"} {
		b.WriteString(`<th class="px-5 py-4 text-left text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">` + template.HTMLEscapeString(h) + `</th>`)
	}
	b.WriteString(`</tr></thead><tbody class="divide-y divide-slate-100 bg-white">`)
	for _, u := range users {
		b.WriteString(`<tr class="align-top">`)
		writeCell(&b, u.Username, "font-medium")
		writeCell(&b, u.Email, "")
		writeCell(&b, u.InternalID, "font-mono text-[12px] text-slate-500")
		writeCell(&b, fmt.Sprintf("%d", len(u.Roles)), "")
		writeBoolCell(&b, u.IsActive)
		writeBoolCell(&b, u.IsSuperUser)
		b.WriteString(`<td class="px-5 py-4"><div class="flex gap-2">`)
		b.WriteString(`<a href="/admin/pages/users?edit=` + template.HTMLEscapeString(u.InternalID) + `" class="inline-flex items-center rounded-lg bg-white px-3 py-2 text-sm font-medium text-slate-700 ring-1 ring-inset ring-slate-300 transition hover:bg-slate-50">Изменить</a>`)
		writeDeleteForm(&b, "/admin/users/delete", "user_id", u.InternalID, "Удалить пользователя "+template.JSEscapeString(u.Username)+"?")
		b.WriteString(`</div></td></tr>`)
	}
	b.WriteString(`</tbody></table></div></div>`)
	return template.HTML(b.String())
}

func renderCreateRoleForm(roles []*models.Role) template.HTML {
	chips := roleChipsJS(roles, "r_parent_roles", "appendParentRole")
	var b strings.Builder
	b.WriteString(`<form method="post" action="/admin/roles/create" data-admin-ajax="true" class="grid gap-4">`)
	b.WriteString(`<div class="grid gap-4 lg:grid-cols-2">`)
	b.WriteString(`<div><label class="mb-2 block text-sm font-medium text-slate-700">Название роли</label><input name="name" placeholder="sai_crud_user_access" required class="` + inputClass() + `"></div>`)
	b.WriteString(`<div><label class="mb-2 block text-sm font-medium text-slate-700">Родительские роли</label>`)
	b.WriteString(`<input id="r_parent_roles" name="parent_roles" placeholder="uuid1, uuid2" class="` + inputClass() + ` font-mono">`)
	b.WriteString(chips)
	b.WriteString(`</div></div>`)
	b.WriteString(permissionsFormHTML("permissions_list_c"))
	b.WriteString(`<div class="flex items-center gap-4"><label class="flex items-center gap-2 text-sm text-slate-700 cursor-pointer"><input type="checkbox" name="is_active" checked class="rounded"> Активна</label>`)
	b.WriteString(`<button type="submit" class="inline-flex h-11 items-center rounded-xl bg-brand-600 px-5 text-sm font-semibold text-white transition hover:bg-brand-500">Создать</button></div>`)
	b.WriteString(`</form>`)
	b.WriteString(string(permissionsScriptHTML("permissions_list_c", "appendParentRole", "r_parent_roles")))
	return template.HTML(b.String())
}

func renderEditRoleForm(role *models.Role, allRoles []*models.Role) template.HTML {
	otherRoles := make([]*models.Role, 0, len(allRoles))
	for _, r := range allRoles {
		if r.InternalID != role.InternalID {
			otherRoles = append(otherRoles, r)
		}
	}
	chips := roleChipsJS(otherRoles, "er_parent_roles", "appendEditParentRole")

	roleID := template.HTMLEscapeString(role.InternalID)
	roleName := template.HTMLEscapeString(role.Name)
	currentParents := template.HTMLEscapeString(strings.Join(role.ParentRoles, ", "))
	activeChecked := ""
	if role.IsActive {
		activeChecked = " checked"
	}

	var b strings.Builder
	b.WriteString(`<form method="post" action="/admin/roles/update" data-admin-ajax="true" class="grid gap-4">`)
	b.WriteString(`<input type="hidden" name="role_id" value="` + roleID + `">`)
	b.WriteString(`<div class="grid gap-4 lg:grid-cols-2">`)
	b.WriteString(`<div><label class="mb-2 block text-sm font-medium text-slate-700">Название роли</label><input name="name" value="` + roleName + `" required class="` + inputClass() + `"></div>`)
	b.WriteString(`<div><label class="mb-2 block text-sm font-medium text-slate-700">Родительские роли</label>`)
	b.WriteString(`<input id="er_parent_roles" name="parent_roles" value="` + currentParents + `" placeholder="uuid1, uuid2" class="` + inputClass() + ` font-mono">`)
	b.WriteString(chips)
	b.WriteString(`</div></div>`)
	b.WriteString(permissionsFormHTMLWithData("permissions_list_e", role.Permissions))
	b.WriteString(`<div class="flex items-center gap-4"><label class="flex items-center gap-2 text-sm text-slate-700 cursor-pointer"><input type="checkbox" name="is_active"` + activeChecked + ` class="rounded"> Активна</label>`)
	b.WriteString(`<button type="submit" class="inline-flex h-11 items-center rounded-xl bg-brand-600 px-5 text-sm font-semibold text-white transition hover:bg-brand-500">Сохранить</button>`)
	b.WriteString(`<a href="/admin/pages/roles" class="inline-flex h-11 items-center rounded-xl bg-slate-100 px-5 text-sm font-medium text-slate-700 transition hover:bg-slate-200">Отмена</a></div>`)
	b.WriteString(`</form>`)
	b.WriteString(string(permissionsScriptHTML("permissions_list_e", "appendEditParentRole", "er_parent_roles")))
	return template.HTML(b.String())
}

func permissionsFormHTML(listID string) string {
	return `
  <div>
    <div class="mb-3 flex items-center justify-between">
      <span class="text-sm font-medium text-slate-700">Permissions</span>
      <button type="button" onclick="addPerm_` + listID + `()" class="inline-flex items-center rounded-lg bg-slate-100 px-3 py-1.5 text-sm font-medium text-slate-700 ring-1 ring-slate-200 hover:bg-slate-200 transition">+ Добавить</button>
    </div>
    <div id="` + listID + `" class="grid gap-3"></div>
  </div>`
}

func permissionsFormHTMLWithData(listID string, perms []models.Permission) string {
	var b strings.Builder
	b.WriteString(`
  <div>
    <div class="mb-3 flex items-center justify-between">
      <span class="text-sm font-medium text-slate-700">Permissions</span>
      <button type="button" onclick="addPerm_` + listID + `()" class="inline-flex items-center rounded-lg bg-slate-100 px-3 py-1.5 text-sm font-medium text-slate-700 ring-1 ring-slate-200 hover:bg-slate-200 transition">+ Добавить</button>
    </div>
    <div id="` + listID + `" class="grid gap-3">`)

	for i, perm := range perms {
		idx := fmt.Sprintf("%d", i)
		ms := template.HTMLEscapeString(perm.Microservice)
		path := template.HTMLEscapeString(perm.Path)
		selectedPost := selectedOpt(perm.Method, "POST")
		selectedGet := selectedOpt(perm.Method, "GET")
		selectedPut := selectedOpt(perm.Method, "PUT")
		selectedDel := selectedOpt(perm.Method, "DELETE")
		selectedPatch := selectedOpt(perm.Method, "PATCH")

		b.WriteString(`<div class="rounded-xl border border-slate-200 bg-slate-50 p-4 grid gap-3">`)
		b.WriteString(`<div class="flex items-center justify-between"><span class="text-xs font-semibold uppercase tracking-wider text-slate-500">Permission #` + fmt.Sprintf("%d", i+1) + `</span>`)
		b.WriteString(`<button type="button" onclick="this.closest('div.rounded-xl').remove()" class="text-xs text-rose-600 hover:text-rose-500">Удалить</button></div>`)
		b.WriteString(`<div class="grid gap-3 lg:grid-cols-3">`)
		b.WriteString(`<div><label class="mb-1 block text-xs font-medium text-slate-600">Microservice</label><input name="perm_microservice_` + idx + `" value="` + ms + `" required class="block h-9 w-full rounded-lg border-0 bg-white px-3 text-sm text-slate-900 ring-1 ring-inset ring-slate-300 focus:ring-2 focus:ring-brand-500"></div>`)
		b.WriteString(`<div><label class="mb-1 block text-xs font-medium text-slate-600">Method</label><select name="perm_method_` + idx + `" class="block h-9 w-full rounded-lg border-0 bg-white px-3 text-sm text-slate-900 ring-1 ring-inset ring-slate-300 focus:ring-2 focus:ring-brand-500"><option` + selectedPost + `>POST</option><option` + selectedGet + `>GET</option><option` + selectedPut + `>PUT</option><option` + selectedDel + `>DELETE</option><option` + selectedPatch + `>PATCH</option></select></div>`)
		b.WriteString(`<div><label class="mb-1 block text-xs font-medium text-slate-600">Path</label><input name="perm_path_` + idx + `" value="` + path + `" required class="block h-9 w-full rounded-lg border-0 bg-white px-3 text-sm text-slate-900 ring-1 ring-inset ring-slate-300 focus:ring-2 focus:ring-brand-500"></div>`)
		b.WriteString(`</div><div class="grid gap-3 lg:grid-cols-2">`)

		b.WriteString(`<div><div class="mb-1 flex items-center justify-between"><label class="text-xs font-medium text-slate-600">required_params</label><button type="button" onclick="addParam_` + listID + `('rp',` + idx + `)" class="text-xs text-brand-600 hover:text-brand-500">+ добавить</button></div><div id="rp_` + idx + `_` + listID + `" class="grid gap-1">`)
		for j, rp := range perm.RequiredParams {
			jdx := fmt.Sprintf("%d", j)
			b.WriteString(`<div class="flex gap-1 items-center"><input name="perm_rp_param_` + idx + `_` + jdx + `" value="` + template.HTMLEscapeString(rp.Param) + `" class="h-8 w-1/2 rounded-lg border-0 bg-white px-2 text-xs font-mono text-slate-900 ring-1 ring-inset ring-slate-300 focus:ring-2 focus:ring-brand-500"><input name="perm_rp_value_` + idx + `_` + jdx + `" value="` + template.HTMLEscapeString(rp.Value) + `" class="h-8 w-1/2 rounded-lg border-0 bg-white px-2 text-xs font-mono text-slate-900 ring-1 ring-inset ring-slate-300 focus:ring-2 focus:ring-brand-500"><button type="button" onclick="this.parentElement.remove()" class="shrink-0 text-rose-400 hover:text-rose-600 text-xs px-1">✕</button></div>`)
		}
		b.WriteString(`</div></div>`)

		b.WriteString(`<div><div class="mb-1 flex items-center justify-between"><label class="text-xs font-medium text-slate-600">restricted_params</label><button type="button" onclick="addParam_` + listID + `('rsp',` + idx + `)" class="text-xs text-brand-600 hover:text-brand-500">+ добавить</button></div><div id="rsp_` + idx + `_` + listID + `" class="grid gap-1">`)
		for j, rsp := range perm.RestrictedParams {
			jdx := fmt.Sprintf("%d", j)
			b.WriteString(`<div class="flex gap-1 items-center"><input name="perm_rsp_param_` + idx + `_` + jdx + `" value="` + template.HTMLEscapeString(rsp.Param) + `" class="h-8 w-1/2 rounded-lg border-0 bg-white px-2 text-xs font-mono text-slate-900 ring-1 ring-inset ring-slate-300 focus:ring-2 focus:ring-brand-500"><input name="perm_rsp_value_` + idx + `_` + jdx + `" value="` + template.HTMLEscapeString(rsp.Value) + `" class="h-8 w-1/2 rounded-lg border-0 bg-white px-2 text-xs font-mono text-slate-900 ring-1 ring-inset ring-slate-300 focus:ring-2 focus:ring-brand-500"><button type="button" onclick="this.parentElement.remove()" class="shrink-0 text-rose-400 hover:text-rose-600 text-xs px-1">✕</button></div>`)
		}
		b.WriteString(`</div></div></div></div>`)
	}

	b.WriteString(`</div></div>`)
	return b.String()
}

func selectedOpt(current, option string) string {
	if current == option {
		return " selected"
	}
	return ""
}

func permissionsScriptHTML(listID, parentFn, parentInputID string) template.HTML {
	return template.HTML(`<script>
var permIdx_` + listID + ` = ` + fmt.Sprintf("%d", 0) + `;
var paramCounts_` + listID + ` = {};
function ` + parentFn + `(id){var el=document.getElementById('` + parentInputID + `');el.value=el.value.trim()?el.value.trim()+', '+id:id;}
function addParam_` + listID + `(kind,pi){
  var key=kind+'_'+pi;
  if(!paramCounts_` + listID + `[key])paramCounts_` + listID + `[key]=0;
  var j=paramCounts_` + listID + `[key]++;
  var c=document.getElementById(kind+'_'+pi+'_` + listID + `');
  var r=document.createElement('div');
  r.className='flex gap-1 items-center';
  r.innerHTML='<input name="perm_'+kind+'_param_'+pi+'_'+j+'" placeholder="filter.user_id" class="h-8 w-1/2 rounded-lg border-0 bg-white px-2 text-xs font-mono text-slate-900 ring-1 ring-inset ring-slate-300 focus:ring-2 focus:ring-brand-500">'+
    '<input name="perm_'+kind+'_value_'+pi+'_'+j+'" placeholder="$.internal_id" class="h-8 w-1/2 rounded-lg border-0 bg-white px-2 text-xs font-mono text-slate-900 ring-1 ring-inset ring-slate-300 focus:ring-2 focus:ring-brand-500">'+
    '<button type="button" onclick="this.parentElement.remove()" class="shrink-0 text-rose-400 hover:text-rose-600 text-xs px-1">✕</button>';
  c.appendChild(r);
}
function addPerm_` + listID + `(){
  var i=permIdx_` + listID + `++;
  var d=document.createElement('div');
  d.className='rounded-xl border border-slate-200 bg-slate-50 p-4 grid gap-3';
  d.innerHTML=
    '<div class="flex items-center justify-between"><span class="text-xs font-semibold uppercase tracking-wider text-slate-500">Permission #'+(i+1)+'</span>'+
    '<button type="button" onclick="this.closest(\'div.rounded-xl\').remove()" class="text-xs text-rose-600 hover:text-rose-500">Удалить</button></div>'+
    '<div class="grid gap-3 lg:grid-cols-3">'+
      '<div><label class="mb-1 block text-xs font-medium text-slate-600">Microservice</label><input name="perm_microservice_'+i+'" placeholder="sai-crud" required class="block h-9 w-full rounded-lg border-0 bg-white px-3 text-sm text-slate-900 ring-1 ring-inset ring-slate-300 focus:ring-2 focus:ring-brand-500"></div>'+
      '<div><label class="mb-1 block text-xs font-medium text-slate-600">Method</label><select name="perm_method_'+i+'" class="block h-9 w-full rounded-lg border-0 bg-white px-3 text-sm text-slate-900 ring-1 ring-inset ring-slate-300 focus:ring-2 focus:ring-brand-500"><option>POST</option><option>GET</option><option>PUT</option><option>DELETE</option><option>PATCH</option></select></div>'+
      '<div><label class="mb-1 block text-xs font-medium text-slate-600">Path</label><input name="perm_path_'+i+'" placeholder="/api/v1" required class="block h-9 w-full rounded-lg border-0 bg-white px-3 text-sm text-slate-900 ring-1 ring-inset ring-slate-300 focus:ring-2 focus:ring-brand-500"></div>'+
    '</div>'+
    '<div class="grid gap-3 lg:grid-cols-2">'+
      '<div><div class="mb-1 flex items-center justify-between"><label class="text-xs font-medium text-slate-600">required_params</label><button type="button" onclick="addParam_` + listID + `(\'rp\','+i+')" class="text-xs text-brand-600 hover:text-brand-500">+ добавить</button></div><div id="rp_'+i+'_` + listID + `" class="grid gap-1"></div></div>'+
      '<div><div class="mb-1 flex items-center justify-between"><label class="text-xs font-medium text-slate-600">restricted_params</label><button type="button" onclick="addParam_` + listID + `(\'rsp\','+i+')" class="text-xs text-brand-600 hover:text-brand-500">+ добавить</button></div><div id="rsp_'+i+'_` + listID + `" class="grid gap-1"></div></div>'+
    '</div>';
  document.getElementById('` + listID + `').appendChild(d);
}
</script>`)
}

func renderRolesTable(roles []*models.Role) template.HTML {
	if len(roles) == 0 {
		return template.HTML(`<div class="rounded-2xl border border-dashed border-slate-300 bg-slate-50 px-6 py-10 text-center text-sm text-slate-500">Роли не найдены.</div>`)
	}

	var b strings.Builder
	b.WriteString(`<div class="overflow-hidden rounded-[22px] border border-slate-200"><div class="overflow-x-auto"><table class="min-w-full divide-y divide-slate-200">`)
	b.WriteString(`<thead class="bg-slate-50"><tr>`)
	for _, h := range []string{"Название", "internal_id", "Разрешений", "Родительских", "Активна", "Действия"} {
		b.WriteString(`<th class="px-5 py-4 text-left text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">` + template.HTMLEscapeString(h) + `</th>`)
	}
	b.WriteString(`</tr></thead><tbody class="divide-y divide-slate-100 bg-white">`)
	for _, r := range roles {
		b.WriteString(`<tr class="align-top">`)
		writeCell(&b, r.Name, "font-medium")
		writeCell(&b, r.InternalID, "font-mono text-[12px] text-slate-500")
		writeCell(&b, fmt.Sprintf("%d", len(r.Permissions)), "")
		writeCell(&b, fmt.Sprintf("%d", len(r.ParentRoles)), "")
		writeBoolCell(&b, r.IsActive)
		b.WriteString(`<td class="px-5 py-4"><div class="flex gap-2">`)
		b.WriteString(`<a href="/admin/pages/roles?edit=` + template.HTMLEscapeString(r.InternalID) + `" class="inline-flex items-center rounded-lg bg-white px-3 py-2 text-sm font-medium text-slate-700 ring-1 ring-inset ring-slate-300 transition hover:bg-slate-50">Изменить</a>`)
		writeDeleteForm(&b, "/admin/roles/delete", "role_id", r.InternalID, "Удалить роль "+template.JSEscapeString(r.Name)+"?")
		b.WriteString(`</div></td></tr>`)
	}
	b.WriteString(`</tbody></table></div></div>`)
	return template.HTML(b.String())
}

func renderTokensTable(tokens []*models.Token) template.HTML {
	if len(tokens) == 0 {
		return template.HTML(`<div class="rounded-2xl border border-dashed border-slate-300 bg-slate-50 px-6 py-10 text-center text-sm text-slate-500">Токены не найдены.</div>`)
	}

	now := time.Now().UnixNano()
	var b strings.Builder
	b.WriteString(`<div class="overflow-hidden rounded-[22px] border border-slate-200"><div class="overflow-x-auto"><table class="min-w-full divide-y divide-slate-200">`)
	b.WriteString(`<thead class="bg-slate-50"><tr>`)
	for _, h := range []string{"User ID", "internal_id", "Access Token", "Истекает", "Статус", "Разрешений", "Действия"} {
		b.WriteString(`<th class="px-5 py-4 text-left text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">` + template.HTMLEscapeString(h) + `</th>`)
	}
	b.WriteString(`</tr></thead><tbody class="divide-y divide-slate-100 bg-white">`)
	for _, t := range tokens {
		b.WriteString(`<tr class="align-top">`)
		writeCell(&b, t.UserID, "font-mono text-[12px]")
		writeCell(&b, t.InternalID, "font-mono text-[12px] text-slate-500")
		writeCell(&b, truncate(t.AccessToken, 20)+"…", "font-mono text-[12px]")
		writeCell(&b, formatNanoTime(t.ExpiresAt), "font-mono text-[12px]")
		writeTokenStatusCell(&b, t.ExpiresAt > now)
		writeCell(&b, fmt.Sprintf("%d", len(t.CompiledPermissions)), "")
		b.WriteString(`<td class="px-5 py-4">`)
		writeDeleteForm(&b, "/admin/tokens/delete", "token_id", t.InternalID, "Инвалидировать токен?")
		b.WriteString(`</td></tr>`)
	}
	b.WriteString(`</tbody></table></div></div>`)
	return template.HTML(b.String())
}

func writeCell(b *strings.Builder, value string, className string) {
	b.WriteString(`<td class="px-5 py-4 text-sm text-slate-700 ` + className + `">`)
	if strings.TrimSpace(value) == "" {
		b.WriteString(`<span class="text-slate-400">—</span>`)
	} else {
		b.WriteString(template.HTMLEscapeString(value))
	}
	b.WriteString(`</td>`)
}

func writeBoolCell(b *strings.Builder, value bool) {
	b.WriteString(`<td class="px-5 py-4 text-sm">`)
	if value {
		b.WriteString(`<span class="inline-flex rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-medium text-emerald-700 ring-1 ring-inset ring-emerald-200">да</span>`)
	} else {
		b.WriteString(`<span class="inline-flex rounded-full bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-500 ring-1 ring-inset ring-slate-200">нет</span>`)
	}
	b.WriteString(`</td>`)
}

func writeTokenStatusCell(b *strings.Builder, active bool) {
	b.WriteString(`<td class="px-5 py-4 text-sm">`)
	if active {
		b.WriteString(`<span class="inline-flex rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-medium text-emerald-700 ring-1 ring-inset ring-emerald-200">активен</span>`)
	} else {
		b.WriteString(`<span class="inline-flex rounded-full bg-rose-100 px-2.5 py-1 text-xs font-medium text-rose-700 ring-1 ring-inset ring-rose-200">истёк</span>`)
	}
	b.WriteString(`</td>`)
}

func writeDeleteForm(b *strings.Builder, action, fieldName, fieldValue, confirmMsg string) {
	b.WriteString(`<form method="post" action="` + action + `" data-admin-ajax="true" data-admin-confirm="` + template.HTMLEscapeString(confirmMsg) + `">`)
	b.WriteString(`<input type="hidden" name="` + fieldName + `" value="` + template.HTMLEscapeString(fieldValue) + `">`)
	b.WriteString(`<button type="submit" class="inline-flex items-center rounded-lg bg-rose-600 px-3 py-2 text-sm font-medium text-white transition hover:bg-rose-500">Удалить</button>`)
	b.WriteString(`</form>`)
}

func formatNanoTime(ns int64) string {
	if ns == 0 {
		return ""
	}
	return time.Unix(0, ns).UTC().Format("2006-01-02 15:04")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
