package device

import (
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// Default threshold for concurrent usage detection (60 seconds)
const defaultConcurrentThreshold = 60 * time.Second

// Config holds device binding configuration
type Config struct {
	Enabled             bool
	MaxDevices          int
	HeaderName          string
	ConcurrentThreshold time.Duration // Time threshold for detecting concurrent usage from different IPs
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
	if config.ConcurrentThreshold <= 0 {
		config.ConcurrentThreshold = defaultConcurrentThreshold
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

		// Log all headers for debugging
		var headers []string
		for key, values := range c.Request.Header {
			for _, v := range values {
				// Mask Authorization header value
				if strings.EqualFold(key, "Authorization") {
					v = "***masked***"
				}
				headers = append(headers, key+"="+v)
			}
		}
		log.Debugf("device-binding: incoming request - key=%s, device_id=%s, type=%s, client_ip=%s, remote_addr=%s, method=%s, path=%s, headers=[%s]",
			MaskKey(apiKey),
			deviceID,
			deviceType,
			c.ClientIP(),
			c.Request.RemoteAddr,
			c.Request.Method,
			c.Request.URL.Path,
			strings.Join(headers, ", "),
		)

		if deviceID == "" {
			// Shouldn't happen, but fallback to allowing
			log.Warnf("device-binding: could not extract device ID for key %s", MaskKey(apiKey))
			c.Next()
			return
		}

		// Check existing binding
		binding, exists := m.store.Get(apiKey)
		currentIP := c.ClientIP()

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

		// Check if banned
		if binding.Banned {
			log.Warnf("device-binding: rejected banned key %s, reason: %s",
				MaskKey(apiKey), binding.BanReason)
			c.AbortWithStatusJSON(403, gin.H{
				"error":   "api_key_banned",
				"message": "This API key has been banned: " + binding.BanReason,
			})
			return
		}

		// Check for concurrent usage from different IPs
		timeSinceLastSeen := time.Since(binding.LastSeen)
		if binding.LastIP != "" && binding.LastIP != currentIP && timeSinceLastSeen < m.config.ConcurrentThreshold {
			// Different IP within short time = suspicious concurrent usage
			reason := "Concurrent usage detected: different IP within " + timeSinceLastSeen.String()
			log.Warnf("device-binding: BANNED key %s - %s (last_ip=%s, current_ip=%s, last_seen=%s ago)",
				MaskKey(apiKey), reason, binding.LastIP, currentIP, timeSinceLastSeen)

			if err := m.store.Ban(apiKey, reason); err != nil {
				log.Errorf("device-binding: failed to ban key %s: %v", MaskKey(apiKey), err)
			}

			c.AbortWithStatusJSON(403, gin.H{
				"error":   "concurrent_usage_detected",
				"message": "Suspicious concurrent usage detected. API key has been banned. Contact admin to unban.",
			})
			return
		}

		// Update last seen with current IP (allow IP changes over time)
		if err := m.store.UpdateLastSeen(apiKey, currentIP); err != nil {
			log.Warnf("device-binding: failed to update last_seen for key %s: %v", MaskKey(apiKey), err)
		}

		c.Next()
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
