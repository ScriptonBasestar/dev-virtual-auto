package lifecycle

import (
	"context"
	"log/slog"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestScriptPlugin_Name(t *testing.T) {
	p := &ScriptPlugin{}
	if p.Name() != "script" {
		t.Errorf("expected 'script', got %q", p.Name())
	}
}

func TestScriptPlugin_Up_NilConfig(t *testing.T) {
	p := &ScriptPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Script: nil},
		Logger: slog.Default(),
	}

	result, err := p.Up(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestScriptPlugin_Up_EmptyScript(t *testing.T) {
	p := &ScriptPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Script: &config.ScriptPluginConfig{Up: ""}},
		Logger: slog.Default(),
	}

	result, err := p.Up(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestScriptPlugin_Down_NilConfig(t *testing.T) {
	p := &ScriptPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Script: nil},
		Logger: slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScriptPlugin_Stop_NilConfig(t *testing.T) {
	p := &ScriptPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Script: nil},
		Logger: slog.Default(),
	}

	err := p.Stop(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScriptPlugin_Status(t *testing.T) {
	p := &ScriptPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{},
		Logger: slog.Default(),
	}

	services, err := p.Status(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if services != nil {
		t.Error("expected nil services for script plugin status")
	}
}

func TestScriptPlugin_DryRun_Up(t *testing.T) {
	p := &ScriptPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Script: &config.ScriptPluginConfig{Up: "echo hello"},
		},
		Env:       env,
		ConfigDir: "/tmp",
		DryRun:    true,
		Logger:    slog.Default(),
	}

	result, err := p.Up(context.Background(), pctx)
	if err != nil {
		t.Fatalf("dry-run up should not fail: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestScriptPlugin_DryRun_Down(t *testing.T) {
	p := &ScriptPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Script: &config.ScriptPluginConfig{Down: "echo bye"},
		},
		Env:       env,
		ConfigDir: "/tmp",
		DryRun:    true,
		Logger:    slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("dry-run down should not fail: %v", err)
	}
}

func TestScriptPlugin_Up_ExecutesCommand(t *testing.T) {
	p := &ScriptPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Script: &config.ScriptPluginConfig{Up: "true"},
		},
		Env:       env,
		ConfigDir: "/tmp",
		Logger:    slog.Default(),
	}

	result, err := p.Up(context.Background(), pctx)
	if err != nil {
		t.Fatalf("script 'true' should succeed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestScriptPlugin_Up_FailingCommand(t *testing.T) {
	p := &ScriptPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Script: &config.ScriptPluginConfig{Up: "false"},
		},
		Env:       env,
		ConfigDir: "/tmp",
		Logger:    slog.Default(),
	}

	_, err := p.Up(context.Background(), pctx)
	if err == nil {
		t.Fatal("script 'false' should fail")
	}
}
