package config

import "testing"

func TestApplyServerSettings_ModeAndAdmin(t *testing.T) {
	ResetForTest()

	applyServerSettings(ServerConfig{
		Mode:          "debug",
		AdminUsername: "root",
		AdminPassword: "secret",
	})

	if runtime.Mode != "debug" {
		t.Fatalf("expected debug mode, got %s", runtime.Mode)
	}
	if runtime.AdminUsername != "root" {
		t.Fatalf("expected admin username root, got %s", runtime.AdminUsername)
	}
	if runtime.AdminPassword != "secret" {
		t.Fatal("expected admin password to be set")
	}
}
