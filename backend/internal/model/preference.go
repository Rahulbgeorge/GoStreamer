package model

import (
	"errors"
	"strings"
)

// Preference represents a key-value pair stored in the database for app configuration.
type Preference struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Validate verifies the preference parameters.
func (p *Preference) Validate() error {
	if strings.TrimSpace(p.Key) == "" {
		return errors.New("preference key is required")
	}
	return nil
}
