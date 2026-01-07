package device

import (
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// Config holds device binding configuration
type Config struct {
	Enabled    bool
	MaxDevices int
	HeaderName string
}

// Middleware checks device bindings for API requests
type Middleware struct {
	store  *Store
	config Config
}

// NewMiddleware creates a new device binding middleware
func NewMiddleware(store *Store, config Config) *Middleware {
	// Apply defaults
	if config.MaxDevices <= 0 {
		config.MaxDevices = 1
	}
	if config.HeaderName == "" {
		config.HeaderName = "X-Device-ID"
	}

	return &Middleware{
		store:  store,
		config: config,
	}
}

// Handler returns the Gin middleware handler
func (m *Middleware) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip if feature is disabled
		if !m.config.Enabled {
			c.Next()
			return
		}

		// Get API key from context (set by auth middleware)
		apiKey := c.GetString("apiKey")
		if apiKey == "" {
			// No API key means auth middleware didn't set it
			// Let the request pass, auth will handle rejection
			c.Next()
			return
		}

		// Extract device ID
		deviceID, deviceType := m.extractDeviceID(c)
		if deviceID == "" {
			// Shouldn't happen, but fallback to allowing
			log.Warnf("device-binding: could not extract device ID for key %s", MaskKey(apiKey))
			c.Next()
			return
		}

		// Check existing binding
		binding, exists := m.store.Get(apiKey)

		if !exists {
			// First use: auto-register device
			if err := m.store.Save(apiKey, deviceID, deviceType); err != nil {
				log.Errorf("device-binding: failed to save binding for key %s: %v", MaskKey(apiKey), err)
				// Allow request to proceed even if save failed
			} else {
				log.Infof("device-binding: new device registered for key %s: %s (%s)",
					MaskKey(apiKey), deviceID, deviceType)
			}
			c.Next()
			return
		}

		// Check if device matches
		if binding.DeviceID == deviceID {
			// Match: update last_seen and allow
			if err := m.store.UpdateLastSeen(apiKey); err != nil {
				log.Warnf("device-binding: failed to update last_seen for key %s: %v", MaskKey(apiKey), err)
			}
			c.Next()
			return
		}

		// Device mismatch: reject
		log.Warnf("device-binding: rejected request for key %s: expected %s, got %s",
			MaskKey(apiKey), binding.DeviceID, deviceID)

		c.AbortWithStatusJSON(403, gin.H{
			"error":   "device_mismatch",
			"message": "This API key is bound to another device. Contact admin to reset.",
		})
	}
}

// extractDeviceID extracts device identifier from request
func (m *Middleware) extractDeviceID(c *gin.Context) (deviceID string, deviceType string) {
	// Priority 1: Client-generated device ID from header
	if id := c.GetHeader(m.config.HeaderName); id != "" {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			return trimmed, "client_id"
		}
	}

	// Fallback: Client IP
	return c.ClientIP(), "ip"
}
