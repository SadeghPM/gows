package config

import (
	"log"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v2"
)

type AppConfig struct {
	Name      string `json:"name" yaml:"name"`
	AppID     string `json:"app_id" yaml:"id"`
	TicketURL string `json:"ticket_url" yaml:"ticket_url"`
	Secret    string `json:"secret" yaml:"secret"`
}

type ServerConfig struct {
	Port          string `yaml:"port"`
	Mode          string `yaml:"mode"`
	AdminUsername string `yaml:"admin_username"`
	AdminPassword string `yaml:"admin_password"`
}

type YAMLConfig struct {
	Server ServerConfig `yaml:"server"`
	Apps   []AppConfig  `yaml:"apps"`
}

type Runtime struct {
	Port          string
	Mode          string
	AdminUsername string
	AdminPassword string
	ConfigPath    string
}

var runtime Runtime

func Load() []AppConfig {
	setDefaults()
	if envPath := strings.TrimSpace(os.Getenv("GOWS_CONFIG_PATH")); envPath != "" {
		runtime.ConfigPath = envPath
	}

	file, err := os.ReadFile(runtime.ConfigPath)
	if err == nil {
		var config YAMLConfig
		if err := yaml.Unmarshal(file, &config); err == nil {
			return loadYAMLConfig(config)
		}
		log.Printf("Failed to parse %s: %v. Falling back to env defaults.", runtime.ConfigPath, err)
	}

	return loadEnvConfig()
}

func Port() string {
	if runtime.Port == "" {
		return "8080"
	}
	return runtime.Port
}

func Mode() string {
	if runtime.Mode == "" {
		return "production"
	}
	return runtime.Mode
}

func AdminUsername() string {
	if runtime.AdminUsername == "" {
		return "admin"
	}
	return runtime.AdminUsername
}

func AdminPassword() string {
	return runtime.AdminPassword
}

func AdminPasswordConfigured() bool {
	return runtime.AdminPassword != ""
}

func ConfigPath() string {
	if runtime.ConfigPath == "" {
		return "config.yaml"
	}
	return runtime.ConfigPath
}

func SetAdminCredentials(username, password string) {
	if strings.TrimSpace(username) != "" {
		runtime.AdminUsername = strings.TrimSpace(username)
	}
	runtime.AdminPassword = password
}

func Persist(apps []AppConfig) error {
	if runtime.ConfigPath == "" {
		runtime.ConfigPath = "config.yaml"
	}

	config := YAMLConfig{
		Server: ServerConfig{
			Port:          runtime.Port,
			Mode:          runtime.Mode,
			AdminUsername: runtime.AdminUsername,
			AdminPassword: runtime.AdminPassword,
		},
		Apps: append([]AppConfig(nil), apps...),
	}

	sort.Slice(config.Apps, func(i, j int) bool {
		return strings.ToLower(config.Apps[i].AppID) < strings.ToLower(config.Apps[j].AppID)
	})

	output, err := yaml.Marshal(&config)
	if err != nil {
		return err
	}
	return os.WriteFile(runtime.ConfigPath, output, 0644)
}

func ResetForTest() {
	setDefaults()
}

func setDefaults() {
	runtime = Runtime{
		Port:          "8080",
		Mode:          "production",
		AdminUsername: "admin",
		AdminPassword: "",
		ConfigPath:    "config.yaml",
	}
}

func loadYAMLConfig(config YAMLConfig) []AppConfig {
	if config.Server.Port != "" {
		runtime.Port = config.Server.Port
	}
	applyServerSettings(config.Server)

	apps := make([]AppConfig, 0, len(config.Apps))
	for _, app := range config.Apps {
		if strings.TrimSpace(app.Name) == "" {
			app.Name = app.AppID
		}
		apps = append(apps, app)
	}

	log.Printf("Loaded %d apps from %s (server port: %s, mode: %s)", len(apps), runtime.ConfigPath, runtime.Port, runtime.Mode)
	return apps
}

func loadEnvConfig() []AppConfig {
	if port := os.Getenv("PORT"); port != "" {
		runtime.Port = port
	}
	applyServerSettings(ServerConfig{
		Mode:          os.Getenv("GOWS_MODE"),
		AdminUsername: os.Getenv("GOWS_ADMIN_USERNAME"),
		AdminPassword: os.Getenv("GOWS_ADMIN_PASSWORD"),
	})

	app := AppConfig{
		Name:      "Default",
		AppID:     os.Getenv("APP_ID"),
		TicketURL: os.Getenv("LARAVEL_TICKET_URL"),
		Secret:    os.Getenv("INTERNAL_SECRET"),
	}
	if app.AppID == "" {
		app.AppID = "default"
	}
	if app.Name == "" {
		app.Name = app.AppID
	}
	if app.TicketURL == "" {
		app.TicketURL = "http://localhost:8000/api/internal/ws/validate-ticket"
	}
	if app.Secret == "" {
		app.Secret = "super-secret-internal-key"
	}

	log.Printf("Loaded single-tenant fallback from env vars (AppID: %s, mode: %s)", app.AppID, runtime.Mode)
	return []AppConfig{app}
}

func applyServerSettings(config ServerConfig) {
	if config.Mode != "" {
		mode := strings.ToLower(strings.TrimSpace(config.Mode))
		if mode == "debug" || mode == "production" {
			runtime.Mode = mode
		} else {
			log.Printf("Invalid server mode %q; using %s", config.Mode, runtime.Mode)
		}
	}
	if config.AdminUsername != "" {
		runtime.AdminUsername = config.AdminUsername
	}
	if config.AdminPassword != "" {
		runtime.AdminPassword = config.AdminPassword
	}
}
