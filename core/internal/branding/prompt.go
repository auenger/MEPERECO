package branding

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

// NormalizeDefaultHelperInstructions updates only the built-in assistant persona.
// Keep user-authored agent instructions, skill identifiers, and paths untouched.
func NormalizeDefaultHelperInstructions(instructions string) string {
	for _, prefix := range []string{
		"You are Sili Agent, the built-in AI assistant for this MEPERStudio workspace.",
		"You are MEPER Agent, the built-in AI assistant for this MEPERStudio workspace.",
		"你是 Sili Agent，这个 MEPERStudio workspace 内置的 AI 助手。",
		"你是 MEPER Agent，这个 MEPERStudio workspace 内置的 AI 助手。",
	} {
		if strings.HasPrefix(instructions, prefix) {
			return BrandCLIText(strings.ReplaceAll(instructions, "Sili Agent", "MEPER Agent"))
		}
	}
	return instructions
}
