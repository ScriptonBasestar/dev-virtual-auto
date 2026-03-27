package lifecycle

import (
	"context"
	"log/slog"
	"reflect"
	"testing"

	"github.com/ScriptonBasestar/dva/internal/config"
)

func TestMultipassPlugin_Name(t *testing.T) {
	p := &MultipassPlugin{}
	if p.Name() != "multipass" {
		t.Errorf("expected 'multipass', got %q", p.Name())
	}
}

func TestMultipassPlugin_Up_NilConfig(t *testing.T) {
	p := &MultipassPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Multipass: nil},
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

func TestMultipassPlugin_Down_NilConfig(t *testing.T) {
	p := &MultipassPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Multipass: nil},
		Logger: slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMultipassPlugin_Stop_NilConfig(t *testing.T) {
	p := &MultipassPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Multipass: nil},
		Logger: slog.Default(),
	}

	err := p.Stop(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMultipassPlugin_Status_NilConfig(t *testing.T) {
	p := &MultipassPlugin{}
	pctx := &PluginContext{
		Entry:  &config.LifecycleEntry{Name: "test-vm", Multipass: nil},
		Logger: slog.Default(),
	}

	services, err := p.Status(context.Background(), pctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}
	if services[0].State != "stopped" {
		t.Errorf("expected 'stopped', got %q", services[0].State)
	}
}

func TestMultipassPlugin_DryRun_Up(t *testing.T) {
	p := &MultipassPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "dev-vm",
			Multipass: &config.MultipassPluginConfig{
				Name:   "dev-vm",
				Image:  "22.04",
				CPUs:   2,
				Memory: "2G",
			},
		},
		Env:       env,
		ConfigDir: "/project",
		DryRun:    true,
		Logger:    slog.Default(),
	}

	result, err := p.Up(context.Background(), pctx)
	if err != nil {
		t.Fatalf("dry-run up should not fail: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result from dry-run")
	}
}

func TestMultipassPlugin_DryRun_Down(t *testing.T) {
	p := &MultipassPlugin{}
	env := config.NewEnvironment(nil, "/tmp", "/tmp")
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "dev-vm",
			Multipass: &config.MultipassPluginConfig{
				Name: "dev-vm",
			},
		},
		Env:       env,
		ConfigDir: "/project",
		DryRun:    true,
		Logger:    slog.Default(),
	}

	err := p.Down(context.Background(), pctx)
	if err != nil {
		t.Fatalf("dry-run down should not fail: %v", err)
	}
}

func TestMultipassPlugin_BuildLaunchArgs_Basic(t *testing.T) {
	p := &MultipassPlugin{}
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "my-vm",
			Multipass: &config.MultipassPluginConfig{
				Name: "my-vm",
			},
		},
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	args := p.buildLaunchArgs(pctx)
	expected := []string{"launch", "--name", "my-vm"}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("expected %v, got %v", expected, args)
	}
}

func TestMultipassPlugin_BuildLaunchArgs_Full(t *testing.T) {
	p := &MultipassPlugin{}
	pctx := &PluginContext{
		Entry: &config.LifecycleEntry{
			Name: "full-vm",
			Multipass: &config.MultipassPluginConfig{
				Name:      "full-vm",
				Image:     "22.04",
				CPUs:      4,
				Memory:    "4G",
				Disk:      "20G",
				CloudInit: "cloud-init.yaml",
			},
		},
		ConfigDir: "/project",
		Logger:    slog.Default(),
	}

	args := p.buildLaunchArgs(pctx)
	expected := []string{
		"launch", "--name", "full-vm",
		"--cpus", "4",
		"--memory", "4G",
		"--disk", "20G",
		"--cloud-init", "/project/cloud-init.yaml",
		"22.04",
	}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("expected %v, got %v", expected, args)
	}
}
