package apps

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sadegh/gows/internal/config"
)

var appsMu sync.RWMutex
var apps = make(map[string]*config.AppConfig)
var hubs = make(map[string]*Hub)
var appStats = make(map[string]*AppStats)

var errAppIDRequired = errors.New("app id is required")
var errTicketURLRequired = errors.New("ticket_url is required")

// AppStats tracks per-app broadcast counters.
type AppStats struct {
	BroadcastRequests atomic.Uint64
	BroadcastSent     atomic.Uint64
	BroadcastFailed   atomic.Uint64
}

// AppStatsSnapshot is a read-only copy of app stats.
type AppStatsSnapshot struct {
	BroadcastRequests uint64
	BroadcastSent     uint64
	BroadcastFailed   uint64
}

type AppSnapshot struct {
	App         config.AppConfig
	Users       int
	Connections int
	Channels    int
	Broadcasts  AppStatsSnapshot
}

func LoadFromConfig(appConfigs []config.AppConfig) {
	appsMu.Lock()
	defer appsMu.Unlock()

	apps = make(map[string]*config.AppConfig)
	hubs = make(map[string]*Hub)
	appStats = make(map[string]*AppStats)

	for _, app := range appConfigs {
		if strings.TrimSpace(app.Name) == "" {
			app.Name = app.AppID
		}
		appCopy := app
		apps[app.AppID] = &appCopy
		hubs[app.AppID] = NewHub()
		ensureAppStatsLocked(app.AppID)
	}
}

func ResetForTest() {
	appsMu.Lock()
	apps = make(map[string]*config.AppConfig)
	hubs = make(map[string]*Hub)
	appStats = make(map[string]*AppStats)
	appsMu.Unlock()
}

func SnapshotAll() []AppSnapshot {
	appsMu.RLock()
	defer appsMu.RUnlock()

	snapshots := make([]AppSnapshot, 0, len(apps))
	for appID, app := range apps {
		hub := hubs[appID]
		if hub == nil {
			continue
		}
		usage := hub.Snapshot()
		statsSnapshot := AppStatsSnapshot{}
		if stats := appStats[appID]; stats != nil {
			statsSnapshot = AppStatsSnapshot{
				BroadcastRequests: stats.BroadcastRequests.Load(),
				BroadcastSent:     stats.BroadcastSent.Load(),
				BroadcastFailed:   stats.BroadcastFailed.Load(),
			}
		}
		appCopy := *app
		snapshots = append(snapshots, AppSnapshot{
			App:         appCopy,
			Users:       usage.Users,
			Connections: usage.Connections,
			Channels:    usage.Channels,
			Broadcasts:  statsSnapshot,
		})
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return strings.ToLower(snapshots[i].App.AppID) < strings.ToLower(snapshots[j].App.AppID)
	})

	return snapshots
}

func ListConfigs() []config.AppConfig {
	appsMu.RLock()
	defer appsMu.RUnlock()

	list := make([]config.AppConfig, 0, len(apps))
	for _, app := range apps {
		appCopy := *app
		list = append(list, appCopy)
	}

	sort.Slice(list, func(i, j int) bool {
		return strings.ToLower(list[i].AppID) < strings.ToLower(list[j].AppID)
	})

	return list
}

func GetIDs() []string {
	appsMu.RLock()
	defer appsMu.RUnlock()

	keys := make([]string, 0, len(apps))
	for k := range apps {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func FindBySecret(secret string) (*config.AppConfig, *Hub, *AppStats) {
	appsMu.RLock()
	defer appsMu.RUnlock()

	for _, app := range apps {
		if subtle.ConstantTimeCompare([]byte(app.Secret), []byte(secret)) == 1 {
			hub := hubs[app.AppID]
			stats := appStats[app.AppID]
			appCopy := *app
			return &appCopy, hub, stats
		}
	}

	return nil, nil, nil
}

func GetByID(appID string) (*config.AppConfig, *Hub, bool) {
	appsMu.RLock()
	defer appsMu.RUnlock()

	app, ok := apps[appID]
	if !ok {
		return nil, nil, false
	}
	hub := hubs[appID]
	appCopy := *app
	return &appCopy, hub, true
}

func EnsureStats(appID string) *AppStats {
	appsMu.Lock()
	defer appsMu.Unlock()
	return ensureAppStatsLocked(appID)
}

func GenerateSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func IsValidAppID(appID string) bool {
	if appID == "" {
		return false
	}
	for _, ch := range appID {
		if ch >= 'a' && ch <= 'z' {
			continue
		}
		if ch >= 'A' && ch <= 'Z' {
			continue
		}
		if ch >= '0' && ch <= '9' {
			continue
		}
		switch ch {
		case '-', '_', '.':
			continue
		default:
			return false
		}
	}
	return true
}

func Add(app config.AppConfig) error {
	appID := strings.TrimSpace(app.AppID)
	if appID == "" {
		return errAppIDRequired
	}
	if !IsValidAppID(appID) {
		return errors.New("app id may only contain letters, numbers, dashes, underscores, and dots")
	}
	app.AppID = appID
	app.TicketURL = strings.TrimSpace(app.TicketURL)
	if app.TicketURL == "" {
		return errTicketURLRequired
	}
	if app.Secret == "" {
		return errors.New("secret is required")
	}
	app.Name = strings.TrimSpace(app.Name)
	if app.Name == "" {
		app.Name = app.AppID
	}

	appsMu.Lock()
	defer appsMu.Unlock()

	if _, exists := apps[app.AppID]; exists {
		return errors.New("app id already exists")
	}
	appCopy := app
	apps[app.AppID] = &appCopy
	if _, ok := hubs[app.AppID]; !ok {
		hubs[app.AppID] = NewHub()
	}
	ensureAppStatsLocked(app.AppID)
	return nil
}

func Update(appID, name, ticketURL string) error {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return errAppIDRequired
	}
	name = strings.TrimSpace(name)
	ticketURL = strings.TrimSpace(ticketURL)
	if ticketURL == "" {
		return errTicketURLRequired
	}
	if name == "" {
		name = appID
	}

	appsMu.Lock()
	defer appsMu.Unlock()

	app, ok := apps[appID]
	if !ok {
		return errors.New("app not found")
	}
	app.Name = name
	app.TicketURL = ticketURL
	return nil
}

func Delete(appID string) error {
	appID = strings.TrimSpace(appID)
	if appID == "" {
		return errAppIDRequired
	}

	appsMu.Lock()
	defer appsMu.Unlock()

	if _, ok := apps[appID]; !ok {
		return errors.New("app not found")
	}
	if hub, ok := hubs[appID]; ok {
		hub.CloseAll()
		delete(hubs, appID)
	}
	delete(apps, appID)
	delete(appStats, appID)
	return nil
}

func ensureAppStatsLocked(appID string) *AppStats {
	if stats, ok := appStats[appID]; ok {
		return stats
	}
	stats := &AppStats{}
	appStats[appID] = stats
	return stats
}
