package config

import (
	"fmt"
	"log/slog"
	"strings"
)

// reservedCommands is the canonical set of built-in DVA command names.
// Use IsReservedCommand or ReservedCommandNames for read-only access.
var reservedCommands = map[string]bool{
	"help": true, "version": true, "ls": true, "compose": true,
	"up": true, "stop": true, "down": true, "build": true, "clean": true,
	"run": true, "provision": true, "validate": true, "manifest": true,
	"ktl": true, "ssh": true, "infra": true, "console": true,
	"completion": true, "cmd": true, "init": true, "status": true, "config": true,
	"logs": true, "restart": true, "show": true, "migrate": true, "doctor": true,
	"dev": true, "app": true,
}

// hookableCommands is the subset of reserved commands that support
// before/replace/after hooks via the interaction section.
var hookableCommands = map[string]bool{
	"up": true, "down": true, "stop": true,
	"restart": true, "build": true, "clean": true,
	"logs": true, "dev": true,
}

// IsHookableCommand reports whether name is a built-in command that
// supports before/replace/after hooks.
func IsHookableCommand(name string) bool {
	return hookableCommands[name]
}

// HookableCommands returns a copy of the hookable command set.
func HookableCommands() map[string]bool {
	cp := make(map[string]bool, len(hookableCommands))
	for k, v := range hookableCommands {
		cp[k] = v
	}
	return cp
}

// ReservedCommands returns a copy of the built-in DVA command set.
// Custom interaction commands in dva.yml must not use these names,
// as they will be silently shadowed by the built-in commands.
func ReservedCommands() map[string]bool {
	cp := make(map[string]bool, len(reservedCommands))
	for k, v := range reservedCommands {
		cp[k] = v
	}
	return cp
}

// IsReservedCommand reports whether name is a built-in DVA command.
func IsReservedCommand(name string) bool {
	return reservedCommands[name]
}

// ReservedCommandConflict represents a conflict between an interaction
// command name and a reserved built-in command.
type ReservedCommandConflict struct {
	Name   string
	Source string // "interaction", "module:<name>", "override"
}

// ValidateReservedCommands checks if any interaction command names
// conflict with reserved built-in command names.
// Commands that define hook fields (before/replace/after) on hookable
// built-in commands are NOT treated as conflicts.
func ValidateReservedCommands(interaction map[string]*InteractionCommand) []ReservedCommandConflict {
	var conflicts []ReservedCommandConflict
	for name, cmd := range interaction {
		if IsReservedCommand(name) {
			// Hookable command with hook fields → valid hook, not a conflict
			if IsHookableCommand(name) && cmd.HasHooks() {
				continue
			}
			conflicts = append(conflicts, ReservedCommandConflict{
				Name:   name,
				Source: "interaction",
			})
		}
	}
	return conflicts
}

// FormatConflictWarnings formats conflict list as warning messages.
func FormatConflictWarnings(conflicts []ReservedCommandConflict) string {
	if len(conflicts) == 0 {
		return ""
	}

	var names []string
	for _, c := range conflicts {
		names = append(names, fmt.Sprintf("'%s'", c.Name))
	}

	return fmt.Sprintf(
		"interaction command %s conflicts with reserved DVA command(s) and will be ignored. "+
			"Rename to avoid shadowing (e.g., 'my-%s' or 'custom-%s')",
		strings.Join(names, ", "),
		conflicts[0].Name,
		conflicts[0].Name,
	)
}

// WarnReservedCommandConflicts logs warnings for any conflicts found.
func WarnReservedCommandConflicts(interaction map[string]*InteractionCommand) []ReservedCommandConflict {
	conflicts := ValidateReservedCommands(interaction)
	if len(conflicts) > 0 {
		slog.Warn(FormatConflictWarnings(conflicts))
	}
	return conflicts
}
