package cli

import "github.com/auenger/MEPERECO/core/internal/branding"

// BrandCLIText retains the CLI-facing entry point; the common implementation
// also serves onboarding and the daemon's runtime brief.
func BrandCLIText(text string) string { return branding.BrandCLIText(text) }
