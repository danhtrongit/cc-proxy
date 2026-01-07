package device

import (
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// Handler handles management API requests for device bindings
type Handler struct {
	store *Store
}

// NewHandler creates a new Handler
func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// GetBindings returns all device bindings or a specific one
// GET /v0/management/device-bindings
// GET /v0/management/device-bindings?api-key=xxx
func (h *Handler) GetBindings(c *gin.Context) {
	apiKey := strings.TrimSpace(c.Query("api-key"))

	if apiKey != "" {
		// Get specific binding
		binding, exists := h.store.Get(apiKey)
		if !exists {
			c.JSON(404, gin.H{
				"error":   "not_found",
				"message": "No device binding found for this API key",
			})
			return
		}
		c.JSON(200, gin.H{
			"api_key": apiKey,
			"binding": binding,
		})
		return
	}

	// Get all bindings
	bindings := h.store.GetAll()
	c.JSON(200, gin.H{
		"bindings": bindings,
	})
}

// DeleteBinding removes device binding(s)
// DELETE /v0/management/device-bindings?api-key=xxx  - reset specific
// DELETE /v0/management/device-bindings              - reset all
func (h *Handler) DeleteBinding(c *gin.Context) {
	apiKey := strings.TrimSpace(c.Query("api-key"))

	if apiKey != "" {
		// Delete specific binding
		deleted, err := h.store.Delete(apiKey)
		if err != nil {
			log.Errorf("device-binding: failed to delete binding for key %s: %v", MaskKey(apiKey), err)
			c.JSON(500, gin.H{
				"error":   "internal_error",
				"message": "Failed to delete device binding",
			})
			return
		}
		if !deleted {
			c.JSON(404, gin.H{
				"error":   "not_found",
				"message": "No device binding found for this API key",
			})
			return
		}

		log.Infof("device-binding: reset device for key %s by admin", MaskKey(apiKey))
		c.JSON(200, gin.H{
			"message": "Device binding reset successfully",
			"api_key": apiKey,
		})
		return
	}

	// Delete all bindings
	if err := h.store.Clear(); err != nil {
		log.Errorf("device-binding: failed to clear all bindings: %v", err)
		c.JSON(500, gin.H{
			"error":   "internal_error",
			"message": "Failed to clear device bindings",
		})
		return
	}

	log.Infof("device-binding: all device bindings cleared by admin")
	c.JSON(200, gin.H{
		"message": "All device bindings cleared successfully",
	})
}

// UnbanKey removes ban from an API key
// POST /v0/management/device-bindings/unban?api-key=xxx
func (h *Handler) UnbanKey(c *gin.Context) {
	apiKey := strings.TrimSpace(c.Query("api-key"))

	if apiKey == "" {
		c.JSON(400, gin.H{
			"error":   "missing_parameter",
			"message": "api-key parameter is required",
		})
		return
	}

	binding, exists := h.store.Get(apiKey)
	if !exists {
		c.JSON(404, gin.H{
			"error":   "not_found",
			"message": "No device binding found for this API key",
		})
		return
	}

	if !binding.Banned {
		c.JSON(400, gin.H{
			"error":   "not_banned",
			"message": "This API key is not banned",
		})
		return
	}

	if err := h.store.Unban(apiKey); err != nil {
		log.Errorf("device-binding: failed to unban key %s: %v", MaskKey(apiKey), err)
		c.JSON(500, gin.H{
			"error":   "internal_error",
			"message": "Failed to unban API key",
		})
		return
	}

	log.Infof("device-binding: unbanned key %s by admin", MaskKey(apiKey))
	c.JSON(200, gin.H{
		"message": "API key unbanned successfully",
		"api_key": apiKey,
	})
}

// RegisterRoutes registers device binding routes on a router group
// The group should already have management authentication middleware applied
func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	group.GET("/device-bindings", h.GetBindings)
	group.DELETE("/device-bindings", h.DeleteBinding)
	group.POST("/device-bindings/unban", h.UnbanKey)
}
