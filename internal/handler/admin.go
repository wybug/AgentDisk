package handler

import (
	"github.com/agentdisk/agent-disk/internal/service"
	"github.com/agentdisk/agent-disk/pkg/jwt"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// AdminHandler handles admin authentication and user management.
type AdminHandler struct {
	adminSvc  *service.AdminService
	jwtSecret string
	expireHrs int
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(adminSvc *service.AdminService, jwtSecret string, expireHours int) *AdminHandler {
	return &AdminHandler{adminSvc: adminSvc, jwtSecret: jwtSecret, expireHrs: expireHours}
}

type adminLoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login handles POST /v1/disk/admin/login.
func (h *AdminHandler) Login(c *gin.Context) {
	var req adminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "username and password required")
		return
	}

	admin, err := h.adminSvc.Login(req.Username, req.Password)
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	token, err := jwt.GenerateAdminToken(h.jwtSecret, admin.Username, admin.Role, h.expireHrs)
	if err != nil {
		response.InternalError(c, "failed to generate token")
		return
	}

	response.OK(c, gin.H{
		"token":    token,
		"username": admin.Username,
		"role":     admin.Role,
	})
}

type createAdminRequest struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required,min=6"`
	Role        string `json:"role"`
	DisplayName string `json:"displayName"`
}

// CreateUser handles POST /v1/disk/admin/users.
func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req createAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "username and password (min 6 chars) required")
		return
	}

	role := req.Role
	if role == "" {
		role = "admin"
	}
	operator := c.GetString("adminUser")

	admin, err := h.adminSvc.CreateAdmin(req.Username, req.Password, role, req.DisplayName, operator)
	if err != nil {
		response.InternalError(c, "failed to create admin user")
		return
	}

	response.Created(c, gin.H{
		"username":    admin.Username,
		"role":        admin.Role,
		"displayName": admin.DisplayName,
	})
}

// ListUsers handles GET /v1/disk/admin/users.
func (h *AdminHandler) ListUsers(c *gin.Context) {
	admins, err := h.adminSvc.ListAdmins()
	if err != nil {
		response.InternalError(c, "failed to list admin users")
		return
	}
	type adminItem struct {
		Username    string `json:"username"`
		Role        string `json:"role"`
		DisplayName string `json:"displayName"`
		IsActive    bool   `json:"isActive"`
		CreatedBy   string `json:"createdBy"`
	}
	items := make([]adminItem, 0, len(admins))
	for _, a := range admins {
		items = append(items, adminItem{
			Username:    a.Username,
			Role:        a.Role,
			DisplayName: a.DisplayName,
			IsActive:    a.IsActive,
			CreatedBy:   a.CreatedBy,
		})
	}
	response.OK(c, items)
}

type changePasswordRequest struct {
	Password string `json:"password" binding:"required,min=6"`
}

// ChangePassword handles PUT /v1/disk/admin/users/:username/password.
func (h *AdminHandler) ChangePassword(c *gin.Context) {
	username := c.Param("username")
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "password (min 6 chars) required")
		return
	}

	if err := h.adminSvc.ChangePassword(username, req.Password); err != nil {
		response.InternalError(c, "failed to change password")
		return
	}
	response.OK(c, gin.H{"message": "password updated"})
}

// DeleteUser handles DELETE /v1/disk/admin/users/:username.
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	username := c.Param("username")
	if err := h.adminSvc.DeleteAdmin(username); err != nil {
		response.InternalError(c, "failed to delete admin user")
		return
	}
	response.OK(c, gin.H{"message": "admin user deleted"})
}

// Dashboard handles GET /v1/disk/admin/dashboard.
func (h *AdminHandler) Dashboard(c *gin.Context) {
	admins, _ := h.adminSvc.ListAdmins()
	response.OK(c, gin.H{
		"adminUser": c.GetString("adminUser"),
		"adminRole": c.GetString("adminRole"),
		"adminCount": len(admins),
	})
}
