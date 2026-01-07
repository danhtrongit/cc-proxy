package device

import (
	"time"
)

// DeviceBinding represents a binding between an API key and a device
type DeviceBinding struct {
	DeviceID  string    `yaml:"device_id" json:"device_id"`
	Type      string    `yaml:"type" json:"type"` // "ip" or "client_id"
	FirstSeen time.Time `yaml:"first_seen" json:"first_seen"`
	LastSeen  time.Time `yaml:"last_seen" json:"last_seen"`
	LastIP    string    `yaml:"last_ip" json:"last_ip"`       // Track last IP for concurrent detection
	Banned    bool      `yaml:"banned" json:"banned"`         // Ban flag
	BanReason string    `yaml:"ban_reason" json:"ban_reason"` // Reason for ban
	BannedAt  time.Time `yaml:"banned_at" json:"banned_at"`   // When banned
}

// DeviceBindings holds all device bindings
type DeviceBindings struct {
	Bindings map[string]DeviceBinding `yaml:"bindings" json:"bindings"`
}

// NewDeviceBindings creates a new DeviceBindings instance
func NewDeviceBindings() *DeviceBindings {
	return &DeviceBindings{
		Bindings: make(map[string]DeviceBinding),
	}
}

// Get returns the binding for an API key, if exists
func (d *DeviceBindings) Get(apiKey string) (DeviceBinding, bool) {
	if d.Bindings == nil {
		return DeviceBinding{}, false
	}
	binding, exists := d.Bindings[apiKey]
	return binding, exists
}

// Set creates or updates a binding for an API key
func (d *DeviceBindings) Set(apiKey, deviceID, deviceType string) {
	if d.Bindings == nil {
		d.Bindings = make(map[string]DeviceBinding)
	}
	now := time.Now()
	d.Bindings[apiKey] = DeviceBinding{
		DeviceID:  deviceID,
		Type:      deviceType,
		FirstSeen: now,
		LastSeen:  now,
		LastIP:    deviceID, // Initially same as deviceID if using IP
	}
}

// UpdateLastSeen updates the last_seen timestamp and IP for an API key
func (d *DeviceBindings) UpdateLastSeen(apiKey string, currentIP string) {
	if d.Bindings == nil {
		return
	}
	if binding, exists := d.Bindings[apiKey]; exists {
		binding.LastSeen = time.Now()
		binding.LastIP = currentIP
		d.Bindings[apiKey] = binding
	}
}

// Ban marks an API key as banned
func (d *DeviceBindings) Ban(apiKey, reason string) {
	if d.Bindings == nil {
		return
	}
	if binding, exists := d.Bindings[apiKey]; exists {
		binding.Banned = true
		binding.BanReason = reason
		binding.BannedAt = time.Now()
		d.Bindings[apiKey] = binding
	}
}

// Unban removes ban from an API key
func (d *DeviceBindings) Unban(apiKey string) {
	if d.Bindings == nil {
		return
	}
	if binding, exists := d.Bindings[apiKey]; exists {
		binding.Banned = false
		binding.BanReason = ""
		binding.BannedAt = time.Time{}
		d.Bindings[apiKey] = binding
	}
}

// Delete removes the binding for an API key
func (d *DeviceBindings) Delete(apiKey string) bool {
	if d.Bindings == nil {
		return false
	}
	if _, exists := d.Bindings[apiKey]; exists {
		delete(d.Bindings, apiKey)
		return true
	}
	return false
}

// Clear removes all bindings
func (d *DeviceBindings) Clear() {
	d.Bindings = make(map[string]DeviceBinding)
}
