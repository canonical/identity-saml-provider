package cmd

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/canonical/identity-saml-provider/internal/domain"
)

// SPOutputFormatter formats output for `sp` subcommands.
type SPOutputFormatter interface {
	// SPRegistered formats successful registration output.
	SPRegistered(w io.Writer, sp *domain.ServiceProvider) error
	// SPError formats an error during registration.
	SPError(w io.Writer, sp *domain.ServiceProvider, err error) error
}

// newSPFormatter returns an SPOutputFormatter for the given format name.
func newSPFormatter(format string) (SPOutputFormatter, error) {
	switch format {
	case "text":
		return &spTextFormatter{}, nil
	case "json":
		return &spJSONFormatter{}, nil
	default:
		return nil, fmt.Errorf("unsupported output format: %q", format)
	}
}

// --- text formatter ---

type spTextFormatter struct{}

func (f *spTextFormatter) SPRegistered(w io.Writer, sp *domain.ServiceProvider) error {
	_, err := fmt.Fprintf(w,
		"✓ Service provider registered successfully!\n  Entity ID:   %s\n  ACS URL:     %s\n  ACS Binding: %s\n",
		sp.EntityID, sp.ACSURL, sp.ACSBinding,
	)
	return err
}

func (f *spTextFormatter) SPError(w io.Writer, sp *domain.ServiceProvider, err error) error {
	_, writeErr := fmt.Fprintf(w, "✗ Failed to register service provider %q: %v\n", sp.EntityID, err)
	if writeErr != nil {
		return writeErr
	}
	return err
}

// --- json formatter ---

type spJSONFormatter struct{}

type spJSONResult struct {
	Status     string `json:"status"`
	EntityID   string `json:"entity_id"`
	ACSURL     string `json:"acs_url"`
	ACSBinding string `json:"acs_binding"`
	Error      string `json:"error,omitempty"`
}

func (f *spJSONFormatter) SPRegistered(w io.Writer, sp *domain.ServiceProvider) error {
	return json.NewEncoder(w).Encode(spJSONResult{
		Status:     "success",
		EntityID:   sp.EntityID,
		ACSURL:     sp.ACSURL,
		ACSBinding: sp.ACSBinding,
	})
}

func (f *spJSONFormatter) SPError(w io.Writer, sp *domain.ServiceProvider, err error) error {
	writeErr := json.NewEncoder(w).Encode(spJSONResult{
		Status:     "error",
		EntityID:   sp.EntityID,
		ACSURL:     sp.ACSURL,
		ACSBinding: sp.ACSBinding,
		Error:      err.Error(),
	})
	if writeErr != nil {
		return writeErr
	}
	return err
}
