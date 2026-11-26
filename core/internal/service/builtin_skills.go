package service

import (
	"embed"
	"io/fs"
	"path"
	"strings"

	skillpkg "github.com/silieco-ai/silieco/core/internal/skill"
	"github.com/silieco-ai/silieco/core/pkg/skillbundle"
)

//go:embed builtin_skills
var builtinSkillsFS embed.FS

const builtinSkillsRoot = "builtin_skills"

// BuiltinSkills returns the platform's built-in skills, embedded at compile
// time. This is the complete catalog, not the default run injection set.
//
// Layout: builtin_skills/<name>/SKILL.md plus optional supporting files. The
// <name> directory carries a "silieco-" prefix so its on-disk slug can never
// collide with a workspace skill a user authored (see writeSkillFiles, which
// derives the skill directory from AgentSkillData.Name).
func (s *AgentRunService) BuiltinSkills() []AgentSkillData {
	return loadBuiltinSkills()
}

// DefaultBuiltinSkills returns only the cross-cutting platform skills. The
// remaining catalog is available on request and SOP stages explicitly select
// their own required skills. Keep persisted builtin IDs stable for old SOPs.
func (s *AgentRunService) DefaultBuiltinSkills() []AgentSkillData {
	return defaultBuiltinSkills()
}

func defaultBuiltinSkills() []AgentSkillData {
	var skills []AgentSkillData
	for _, name := range []string{"silieco-mentioning", "silieco-projects-and-resources", "silieco-working-on-tasks"} {
		if skill, ok := loadBuiltinSkill(name); ok {
			skills = append(skills, skill)
		}
	}
	return skills
}

// BuiltinSkillCatalog returns metadata only; bodies and supporting files are
// read on demand through the authenticated skill detail endpoint.
func (s *AgentRunService) BuiltinSkillCatalog() []AgentSkillRefData {
	refs := make([]AgentSkillRefData, 0)
	for _, skill := range s.BuiltinSkills() {
		_, description := skillpkg.ParseSkillFrontmatter(skill.Content)
		refs = append(refs, AgentSkillRefData{ID: "builtin:" + skill.Name, Source: "builtin", Name: skill.Name, Description: description})
	}
	return refs
}

func (s *AgentRunService) BuiltinSkillByID(id string) (AgentSkillData, bool) {
	for _, skill := range s.BuiltinSkills() {
		if id == "builtin:"+skill.Name {
			_, skill.Description = skillpkg.ParseSkillFrontmatter(skill.Content)
			skill.ID, skill.Source = id, "builtin"
			return skill, true
		}
	}
	return AgentSkillData{}, false
}

// skillIdentity is shared by merging and bundle construction. Persisted
// workspace IDs and builtin names remain stable even if a skill is renamed.
func skillIdentity(skill AgentSkillData) (source, id string) {
	source, id = skill.Source, skill.ID
	if source == "" {
		if id == "" || strings.HasPrefix(id, "builtin:") {
			source = skillbundle.SourceBuiltin
		} else {
			source = skillbundle.SourceWorkspace
		}
	}
	if id == "" && source == skillbundle.SourceBuiltin {
		id = "builtin:" + skill.Name
	}
	return source, id
}

// MergeRunSkills keeps one materialized skill per stable identity. A stage's frozen
// version takes precedence over the live agent/default copy of that skill.
// Both full and slim claims must use this same merge before hashing.
func MergeRunSkills(base, required []AgentSkillData) []AgentSkillData {
	merged := make([]AgentSkillData, 0, len(base)+len(required))
	positions := make(map[string]int, len(base)+len(required))
	for _, group := range [][]AgentSkillData{base, required} {
		for _, skill := range group {
			source, id := skillIdentity(skill)
			key := source + "\x00" + id
			if index, exists := positions[key]; exists {
				merged[index] = skill
			} else {
				positions[key] = len(merged)
				merged = append(merged, skill)
			}
		}
	}
	return merged
}

func loadBuiltinSkills() []AgentSkillData {
	entries, err := fs.ReadDir(builtinSkillsFS, builtinSkillsRoot)
	if err != nil {
		return nil
	}
	var skills []AgentSkillData
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if skill, ok := loadBuiltinSkill(entry.Name()); ok {
			skills = append(skills, skill)
		}
	}
	return skills
}

func loadBuiltinSkill(name string) (AgentSkillData, bool) {
	dir := path.Join(builtinSkillsRoot, name)
	content, err := fs.ReadFile(builtinSkillsFS, path.Join(dir, "SKILL.md"))
	if err != nil {
		// A skill directory without a SKILL.md is malformed — skip it rather
		// than ship an empty skill.
		return AgentSkillData{}, false
	}
	skill := AgentSkillData{Name: name, Content: string(content)}
	// Any other file in the directory becomes a supporting file, preserving
	// its relative path so subdirectories (e.g. rules/styling.md) survive.
	_ = fs.WalkDir(builtinSkillsFS, dir, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		rel := strings.TrimPrefix(p, dir+"/")
		if rel == "SKILL.md" {
			return nil
		}
		data, readErr := fs.ReadFile(builtinSkillsFS, p)
		if readErr != nil {
			return nil
		}
		skill.Files = append(skill.Files, AgentSkillFileData{Path: rel, Content: string(data)})
		return nil
	})
	return skill, true
}
