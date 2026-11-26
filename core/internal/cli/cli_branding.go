package cli

import (
	"regexp"
	"strings"
)

// BrandCLIText changes command examples without renaming existing skill IDs,
// filesystem paths or API identifiers. Public environment names are mirrored.
var legacyCLIInPrompt = regexp.MustCompile("(^|[^a-zA-Z0-9_./-])silieco([ \\t\\r\\n`]|\\.exe\\b)")

func BrandCLIText(text string) string {
	return strings.ReplaceAll(legacyCLIInPrompt.ReplaceAllString(text, "${1}meperstudio${2}"), "SILIECO_", "MEPERSTUDIO_")
}
