package sessions

import (
	"path/filepath"
	"testing"
	"time"

	"hosts/internal/network"
)

func TestManagerPersistsStableIdentifier(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "sessions.json")

	manager, err := NewManager(time.Minute, "manual", &network.NoopController{}, statePath)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	created, err := manager.Ensure(ClientInfo{IP: "192.168.137.20", MAC: "aa:bb:cc:dd:ee:ff"})
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}
	if len(created.ID) != 6 {
		t.Fatalf("expected 6-char id, got %q", created.ID)
	}
	if created.Identifier == "" || created.Animal == "" {
		t.Fatalf("expected identifier and animal folder, got %#v", created)
	}

	restoredManager, err := NewManager(time.Minute, "manual", &network.NoopController{}, statePath)
	if err != nil {
		t.Fatalf("restore manager: %v", err)
	}

	restored, err := restoredManager.Ensure(ClientInfo{SessionID: created.ID, IP: "192.168.137.45"})
	if err != nil {
		t.Fatalf("ensure restored session: %v", err)
	}
	if restored.ID != created.ID {
		t.Fatalf("expected same session id, got %q want %q", restored.ID, created.ID)
	}
	if restored.Identifier != created.Identifier {
		t.Fatalf("expected same identifier, got %q want %q", restored.Identifier, created.Identifier)
	}
	if restored.Animal != created.Animal {
		t.Fatalf("expected same folder, got %q want %q", restored.Animal, created.Animal)
	}
	if restored.InternetEnabled {
		t.Fatalf("expected restored session to start without active internet")
	}
}

func TestRotateAnimalKeepsShortID(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "sessions.json")

	manager, err := NewManager(time.Minute, "manual", &network.NoopController{}, statePath)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}

	created, err := manager.Ensure(ClientInfo{IP: "192.168.137.20"})
	if err != nil {
		t.Fatalf("ensure session: %v", err)
	}

	rotated, err := manager.RotateAnimal(ClientInfo{SessionID: created.ID, IP: created.IP})
	if err != nil {
		t.Fatalf("rotate animal: %v", err)
	}
	if rotated.ID != created.ID {
		t.Fatalf("expected same id after rotation, got %q want %q", rotated.ID, created.ID)
	}
	if rotated.Identifier == created.Identifier {
		t.Fatalf("expected a new identifier, got %q", rotated.Identifier)
	}
	if len(rotated.ID) != 6 {
		t.Fatalf("expected 6-char id after rotation, got %q", rotated.ID)
	}
}
