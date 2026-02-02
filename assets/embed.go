package assets

import "embed"

// SkillsFS contains built-in skills shipped with mistermorph (under assets/skills).
//
//go:embed skills/**
var SkillsFS embed.FS
