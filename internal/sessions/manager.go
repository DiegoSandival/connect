package sessions

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	mathrand "math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"hosts/internal/network"
)

type animalProfile struct {
	Emoji  string
	Name   string
	Folder string
}

var animalPool = []animalProfile{
	{Emoji: "🐶", Name: "Perro", Folder: "perro"},
	{Emoji: "🐱", Name: "Gato", Folder: "gato"},
	{Emoji: "🐭", Name: "Raton", Folder: "raton"},
	{Emoji: "🐹", Name: "Hamster", Folder: "hamster"},
	{Emoji: "🐰", Name: "Conejo", Folder: "conejo"},
	{Emoji: "🦊", Name: "Zorro", Folder: "zorro"},
	{Emoji: "🐻", Name: "Oso", Folder: "oso"},
	{Emoji: "🐼", Name: "Panda", Folder: "panda"},
	{Emoji: "🐨", Name: "Koala", Folder: "koala"},
	{Emoji: "🐯", Name: "Tigre", Folder: "tigre"},
	{Emoji: "🦁", Name: "Leon", Folder: "leon"},
	{Emoji: "🐮", Name: "Vaca", Folder: "vaca"},
	{Emoji: "🐷", Name: "Cerdo", Folder: "cerdo"},
	{Emoji: "🐸", Name: "Rana", Folder: "rana"},
	{Emoji: "🐵", Name: "Mono", Folder: "mono"},
	{Emoji: "🐔", Name: "Gallina", Folder: "gallina"},
	{Emoji: "🐧", Name: "Pinguino", Folder: "pinguino"},
	{Emoji: "🐦", Name: "Pajaro", Folder: "pajaro"},
	{Emoji: "🐤", Name: "Pollito", Folder: "pollito"},
	{Emoji: "🦆", Name: "Pato", Folder: "pato"},
	{Emoji: "🦅", Name: "Aguila", Folder: "aguila"},
	{Emoji: "🦉", Name: "Buho", Folder: "buho"},
	{Emoji: "🦇", Name: "Murcielago", Folder: "murcielago"},
	{Emoji: "🐺", Name: "Lobo", Folder: "lobo"},
	{Emoji: "🐗", Name: "Jabali", Folder: "jabali"},
	{Emoji: "🐴", Name: "Caballo", Folder: "caballo"},
	{Emoji: "🦄", Name: "Unicornio", Folder: "unicornio"},
	{Emoji: "🐝", Name: "Abeja", Folder: "abeja"},
	{Emoji: "🐛", Name: "Oruga", Folder: "oruga"},
	{Emoji: "🦋", Name: "Mariposa", Folder: "mariposa"},
	{Emoji: "🐌", Name: "Caracol", Folder: "caracol"},
	{Emoji: "🐞", Name: "Mariquita", Folder: "mariquita"},
	{Emoji: "🐢", Name: "Tortuga", Folder: "tortuga"},
	{Emoji: "🐍", Name: "Serpiente", Folder: "serpiente"},
	{Emoji: "🦎", Name: "Lagarto", Folder: "lagarto"},
	{Emoji: "🦖", Name: "Dinosaurio", Folder: "dinosaurio"},
	{Emoji: "🦕", Name: "Dinosaurio Largo", Folder: "dinosaurio-largo"},
	{Emoji: "🐙", Name: "Pulpo", Folder: "pulpo"},
	{Emoji: "🦑", Name: "Calamar", Folder: "calamar"},
	{Emoji: "🦐", Name: "Camaron", Folder: "camaron"},
	{Emoji: "🦞", Name: "Langosta", Folder: "langosta"},
	{Emoji: "🦀", Name: "Cangrejo", Folder: "cangrejo"},
	{Emoji: "🐠", Name: "Pez", Folder: "pez"},
	{Emoji: "🐟", Name: "Pez Azul", Folder: "pez-azul"},
	{Emoji: "🐡", Name: "Pez Globo", Folder: "pez-globo"},
	{Emoji: "🐬", Name: "Delfin", Folder: "delfin"},
	{Emoji: "🐳", Name: "Ballena", Folder: "ballena"},
	{Emoji: "🦈", Name: "Tiburon", Folder: "tiburon"},
}

type ClientInfo struct {
	IP        string `json:"ip"`
	MAC       string `json:"mac,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

type Snapshot struct {
	ID               string     `json:"id,omitempty"`
	Identifier       string     `json:"identifier,omitempty"`
	IP               string     `json:"ip"`
	MAC              string     `json:"mac,omitempty"`
	Animal           string     `json:"animal,omitempty"`
	AnimalName       string     `json:"animal_name,omitempty"`
	AnimalEmoji      string     `json:"animal_emoji,omitempty"`
	InternetEnabled  bool       `json:"internet_enabled"`
	FirstSeenAt      time.Time  `json:"first_seen_at"`
	LastSeenAt       time.Time  `json:"last_seen_at"`
	ActivatedAt      *time.Time `json:"activated_at,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	LastUploadAt     *time.Time `json:"last_upload_at,omitempty"`
	SecondsRemaining int        `json:"seconds_remaining"`
}

type trackedSession struct {
	snapshot Snapshot
	blocked  bool
	timer    *time.Timer
}

type persistedState struct {
	Sessions []Snapshot `json:"sessions"`
}

type Manager struct {
	mu         sync.Mutex
	sessions   map[string]*trackedSession
	clientRefs map[string]string
	duration   time.Duration
	mode       string
	controller network.AccessController
	rng        *mathrand.Rand
	statePath  string
}

func NewManager(duration time.Duration, mode string, controller network.AccessController, statePath string) (*Manager, error) {
	if mode != "auto" {
		mode = "manual"
	}

	m := &Manager{
		sessions:   make(map[string]*trackedSession),
		clientRefs: make(map[string]string),
		duration:   duration,
		mode:       mode,
		controller: controller,
		rng:        mathrand.New(mathrand.NewSource(time.Now().UnixNano())),
		statePath:  strings.TrimSpace(statePath),
	}

	if err := m.loadState(); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Manager) Touch(ctx context.Context, client ClientInfo) (Snapshot, error) {
	m.mu.Lock()
	tracked := m.getOrCreateLocked(client)
	now := time.Now()
	m.refreshClientLocked(tracked, client, now)
	shouldAutoActivate := m.mode == "auto" && !tracked.snapshot.InternetEnabled
	shouldBlock := m.mode == "manual" && !tracked.snapshot.InternetEnabled && !tracked.blocked
	identifier := tracked.snapshot.ID

	if !shouldAutoActivate && !shouldBlock {
		if err := m.persistLocked(); err != nil {
			m.mu.Unlock()
			return Snapshot{}, err
		}
		snapshot := m.snapshotLocked(identifier)
		m.mu.Unlock()
		return snapshot, nil
	}
	m.mu.Unlock()

	if shouldAutoActivate {
		return m.Activate(ctx, client)
	}

	if err := m.controller.EnsureBlocked(ctx, client.IP); err != nil {
		return Snapshot{}, err
	}

	m.mu.Lock()
	if tracked = m.sessions[identifier]; tracked != nil {
		tracked.blocked = true
		if err := m.persistLocked(); err != nil {
			m.mu.Unlock()
			return Snapshot{}, err
		}
	}
	snapshot := m.snapshotLocked(identifier)
	m.mu.Unlock()
	return snapshot, nil
}

func (m *Manager) Activate(ctx context.Context, client ClientInfo) (Snapshot, error) {
	if err := m.controller.AllowClient(ctx, client.IP); err != nil {
		return Snapshot{}, err
	}

	now := time.Now()
	expiresAt := now.Add(m.duration)

	m.mu.Lock()
	tracked := m.getOrCreateLocked(client)
	if tracked.timer != nil {
		tracked.timer.Stop()
	}
	m.refreshClientLocked(tracked, client, now)
	tracked.snapshot.InternetEnabled = true
	tracked.snapshot.ActivatedAt = &now
	tracked.snapshot.ExpiresAt = &expiresAt
	tracked.blocked = false
	tracked.timer = time.AfterFunc(time.Until(expiresAt), func() {
		_ = m.controller.BlockClient(context.Background(), client.IP)
		m.expire(tracked.snapshot.ID)
	})
	if err := m.persistLocked(); err != nil {
		m.mu.Unlock()
		return Snapshot{}, err
	}
	snapshot := m.snapshotLocked(tracked.snapshot.ID)
	m.mu.Unlock()

	return snapshot, nil
}

func (m *Manager) Ensure(client ClientInfo) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tracked := m.getOrCreateLocked(client)
	m.refreshClientLocked(tracked, client, time.Now())
	if err := m.persistLocked(); err != nil {
		return Snapshot{}, err
	}
	return m.snapshotLocked(tracked.snapshot.ID), nil
}

func (m *Manager) MarkUpload(client ClientInfo) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tracked := m.getOrCreateLocked(client)
	now := time.Now()
	m.refreshClientLocked(tracked, client, now)
	tracked.snapshot.LastUploadAt = &now
	if err := m.persistLocked(); err != nil {
		return Snapshot{}, err
	}
	return m.snapshotLocked(tracked.snapshot.ID), nil
}

func (m *Manager) RotateAnimal(client ClientInfo) (Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tracked := m.getOrCreateLocked(client)
	previous := tracked.snapshot
	profile := m.nextAnimalLocked(tracked.snapshot.AnimalName)
	tracked.snapshot.LastSeenAt = time.Now()
	m.applyAnimalLocked(&tracked.snapshot, profile)
	m.refreshClientLocked(tracked, client, tracked.snapshot.LastSeenAt)
	if err := m.persistLocked(); err != nil {
		tracked.snapshot = previous
		return Snapshot{}, err
	}
	return m.snapshotLocked(tracked.snapshot.ID), nil
}

func (m *Manager) Status(clientKey string) Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snapshotLocked(m.resolveSessionIDLocked(ClientInfo{SessionID: clientKey, IP: clientKey}))
}

func (m *Manager) List() []Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	result := make([]Snapshot, 0, len(ids))
	for _, id := range ids {
		result = append(result, m.snapshotLocked(id))
	}
	return result
}

func (m *Manager) expire(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tracked, ok := m.sessions[sessionID]
	if !ok {
		return
	}
	tracked.snapshot.InternetEnabled = false
	tracked.snapshot.ExpiresAt = nil
	tracked.blocked = true
	tracked.timer = nil
	_ = m.persistLocked()
}

func (m *Manager) getOrCreateLocked(client ClientInfo) *trackedSession {
	if sessionID := m.resolveSessionIDLocked(client); sessionID != "" {
		tracked := m.sessions[sessionID]
		if tracked != nil {
			m.ensureIdentityLocked(tracked)
			return tracked
		}
	}

	now := time.Now()
	tracked := &trackedSession{
		snapshot: Snapshot{
			ID:          m.nextSessionIDLocked(),
			IP:          strings.TrimSpace(client.IP),
			MAC:         strings.ToLower(strings.TrimSpace(client.MAC)),
			FirstSeenAt: now,
			LastSeenAt:  now,
		},
	}
	m.applyAnimalLocked(&tracked.snapshot, m.nextAnimalLocked(""))
	m.sessions[tracked.snapshot.ID] = tracked
	m.refreshClientLocked(tracked, client, now)
	return tracked
}

func (m *Manager) ensureIdentityLocked(tracked *trackedSession) {
	if tracked.snapshot.ID == "" {
		tracked.snapshot.ID = m.nextSessionIDLocked()
	}
	if tracked.snapshot.Animal == "" || tracked.snapshot.Identifier == "" || tracked.snapshot.AnimalName == "" || tracked.snapshot.AnimalEmoji == "" {
		m.applyAnimalLocked(&tracked.snapshot, m.nextAnimalLocked(tracked.snapshot.AnimalName))
	}
	if tracked.snapshot.FirstSeenAt.IsZero() {
		tracked.snapshot.FirstSeenAt = time.Now()
	}
	if tracked.snapshot.LastSeenAt.IsZero() {
		tracked.snapshot.LastSeenAt = tracked.snapshot.FirstSeenAt
	}
	if tracked.snapshot.MAC != "" {
		tracked.snapshot.MAC = strings.ToLower(strings.TrimSpace(tracked.snapshot.MAC))
	}
	if tracked.snapshot.InternetEnabled && tracked.snapshot.ExpiresAt != nil && time.Now().After(*tracked.snapshot.ExpiresAt) {
		tracked.snapshot.InternetEnabled = false
		tracked.snapshot.ExpiresAt = nil
		tracked.snapshot.ActivatedAt = nil
	}
	tracked.snapshot.SecondsRemaining = 0
	m.rememberClientLocked(tracked.snapshot)
}

func (m *Manager) resolveSessionIDLocked(client ClientInfo) string {
	sessionID := normalizeID(client.SessionID)
	if sessionID != "" {
		if _, ok := m.sessions[sessionID]; ok {
			return sessionID
		}
	}
	if client.MAC != "" {
		if sessionID = m.clientRefs[indexKey("mac", client.MAC)]; sessionID != "" {
			if _, ok := m.sessions[sessionID]; ok {
				return sessionID
			}
		}
	}
	if client.IP != "" {
		if sessionID = m.clientRefs[indexKey("ip", client.IP)]; sessionID != "" {
			if _, ok := m.sessions[sessionID]; ok {
				return sessionID
			}
		}
	}
	return ""
}

func (m *Manager) refreshClientLocked(tracked *trackedSession, client ClientInfo, seenAt time.Time) {
	tracked.snapshot.LastSeenAt = seenAt
	if ip := strings.TrimSpace(client.IP); ip != "" {
		tracked.snapshot.IP = ip
	}
	if mac := strings.ToLower(strings.TrimSpace(client.MAC)); mac != "" {
		tracked.snapshot.MAC = mac
	}
	m.rememberClientLocked(tracked.snapshot)
}

func (m *Manager) rememberClientLocked(snapshot Snapshot) {
	if snapshot.ID != "" {
		m.clientRefs[indexKey("session", snapshot.ID)] = snapshot.ID
	}
	if snapshot.MAC != "" {
		m.clientRefs[indexKey("mac", snapshot.MAC)] = snapshot.ID
	}
	if snapshot.IP != "" {
		m.clientRefs[indexKey("ip", snapshot.IP)] = snapshot.ID
	}
}

func (m *Manager) nextAnimalLocked(currentName string) animalProfile {
	if len(animalPool) == 0 {
		return animalProfile{Emoji: "🐾", Name: "Animal", Folder: "animal"}
	}

	used := make(map[string]struct{}, len(m.sessions))
	for _, tracked := range m.sessions {
		if tracked == nil || tracked.snapshot.AnimalName == "" || tracked.snapshot.AnimalName == currentName {
			continue
		}
		used[tracked.snapshot.AnimalName] = struct{}{}
	}

	candidates := make([]animalProfile, 0, len(animalPool))
	for _, animal := range animalPool {
		if animal.Name == currentName {
			continue
		}
		if _, ok := used[animal.Name]; ok {
			continue
		}
		candidates = append(candidates, animal)
	}
	if len(candidates) == 0 {
		for _, animal := range animalPool {
			if animal.Name == currentName && len(animalPool) > 1 {
				continue
			}
			candidates = append(candidates, animal)
		}
	}
	if len(candidates) == 0 {
		return animalPool[0]
	}
	return candidates[m.rng.Intn(len(candidates))]
}

func (m *Manager) applyAnimalLocked(snapshot *Snapshot, profile animalProfile) {
	snapshot.Animal = buildAnimalFolder(profile.Folder, snapshot.ID)
	snapshot.Identifier = buildAnimalFolder(profile.Folder, snapshot.ID)
	snapshot.AnimalName = profile.Name
	snapshot.AnimalEmoji = profile.Emoji
}

func (m *Manager) nextSessionIDLocked() string {
	for {
		candidate := randomShortID()
		if candidate == "" {
			candidate = fmt.Sprintf("%06x", m.rng.Uint32()&0xFFFFFF)
		}
		if _, ok := m.sessions[candidate]; !ok {
			return candidate
		}
	}
}

func (m *Manager) snapshotLocked(sessionID string) Snapshot {
	tracked, ok := m.sessions[sessionID]
	if !ok {
		return Snapshot{ID: normalizeID(sessionID), IP: sessionID}
	}
	snapshot := tracked.snapshot
	if snapshot.InternetEnabled && snapshot.ExpiresAt != nil {
		remaining := int(time.Until(*snapshot.ExpiresAt).Seconds())
		if remaining < 0 {
			remaining = 0
		}
		snapshot.SecondsRemaining = remaining
	}
	return snapshot
}

func (m *Manager) loadState() error {
	if m.statePath == "" {
		return nil
	}

	data, err := os.ReadFile(m.statePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read session state: %w", err)
	}

	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("decode session state: %w", err)
	}

	for _, snapshot := range state.Sessions {
		snapshot.ID = normalizeID(snapshot.ID)
		if snapshot.ID == "" {
			snapshot.ID = m.nextSessionIDLocked()
		}
		if snapshot.FirstSeenAt.IsZero() {
			snapshot.FirstSeenAt = time.Now()
		}
		if snapshot.LastSeenAt.IsZero() {
			snapshot.LastSeenAt = snapshot.FirstSeenAt
		}
		snapshot.InternetEnabled = false
		snapshot.ActivatedAt = nil
		snapshot.ExpiresAt = nil
		snapshot.SecondsRemaining = 0

		tracked := &trackedSession{snapshot: snapshot}
		m.ensureIdentityLocked(tracked)
		m.sessions[tracked.snapshot.ID] = tracked
	}

	return nil
}

func (m *Manager) persistLocked() error {
	if m.statePath == "" {
		return nil
	}

	state := persistedState{Sessions: make([]Snapshot, 0, len(m.sessions))}
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		snapshot := m.snapshotLocked(id)
		snapshot.SecondsRemaining = 0
		state.Sessions = append(state.Sessions, snapshot)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(m.statePath), 0o755); err != nil {
		return fmt.Errorf("create session state dir: %w", err)
	}

	tempPath := m.statePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return fmt.Errorf("write session state: %w", err)
	}
	if err := os.Rename(tempPath, m.statePath); err != nil {
		return fmt.Errorf("replace session state: %w", err)
	}
	return nil
}

func buildAnimalFolder(folder, sessionID string) string {
	base := strings.TrimSpace(folder)
	if base == "" {
		base = "animal"
	}
	return fmt.Sprintf("%s%s", base, normalizeID(sessionID))
}

func normalizeID(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) > 6 {
		value = value[:6]
	}
	return value
}

func indexKey(kind, value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	return kind + ":" + value
}

func randomShortID() string {
	buffer := make([]byte, 3)
	if _, err := cryptorand.Read(buffer); err != nil {
		return ""
	}
	return hex.EncodeToString(buffer)
}
