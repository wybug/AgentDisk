package handler

import (
	"github.com/agentdisk/agent-disk/internal/feature"
	"github.com/agentdisk/agent-disk/pkg/response"
	"github.com/gin-gonic/gin"
)

// FeatureHandler exposes the runtime feature flag registry to admins. Routes
// mount under /v1/disk/admin/features so they inherit AdminAuth + AdminOnly.
type FeatureHandler struct {
	reg *feature.Registry
}

// NewFeatureHandler wires the registry. reg must be the same instance the
// middleware reads from, otherwise admin toggles would not affect live
// traffic.
func NewFeatureHandler(reg *feature.Registry) *FeatureHandler {
	return &FeatureHandler{reg: reg}
}

// featureUpdateRequest is the PATCH body. Name is the wire-format flag name
// (e.g. "okfReader"); Enabled is the desired state.
type featureUpdateRequest struct {
	Name    string `json:"name" binding:"required"`
	Enabled bool   `json:"enabled"`
}

// List returns all known flags with their current on/off state. Mounted at
// GET /v1/disk/admin/features.
func (h *FeatureHandler) List(c *gin.Context) {
	response.OK(c, gin.H{"features": h.reg.FlagsList()})
}

// Update flips a single flag. Mounted at PATCH /v1/disk/admin/features.
// On success the registry has been updated AND the change has been persisted
// to config.yaml, so a process restart keeps the new value.
func (h *FeatureHandler) Update(c *gin.Context) {
	var req featureUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request: "+err.Error())
		return
	}
	name := feature.FlagName(req.Name)
	if err := h.reg.Set(name, req.Enabled); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c, gin.H{"features": h.reg.FlagsList()})
}
