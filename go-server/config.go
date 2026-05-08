package main

import (
	"log"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

type AppConfig struct {
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

var serverPort = "8080"
var serverMode = "production"
var adminUsername = "admin"
var adminPassword string

func initConfig() {
	stats.startedAt = time.Now()

	file, err := os.ReadFile("config.yaml")
	if err == nil {
		var config YAMLConfig
		if err := yaml.Unmarshal(file, &config); err == nil {
			loadYAMLConfig(config)
			return
		}
		log.Printf("Failed to parse config.yaml: %v. Falling back to env defaults.", err)
	}

	loadEnvConfig()
}

func loadYAMLConfig(config YAMLConfig) {
	if config.Server.Port != "" {
		serverPort = config.Server.Port
	}
	applyServerSettings(config.Server)

	for _, app := range config.Apps {
		a := app
		apps[a.AppID] = &a
		hubs[a.AppID] = NewHub()
	}
	infoLog("Loaded %d apps from config.yaml (server port: %s, mode: %s)", len(apps), serverPort, serverMode)
}

func loadEnvConfig() {
	if port := os.Getenv("PORT"); port != "" {
		serverPort = port
	}
	applyServerSettings(ServerConfig{
		Mode:          os.Getenv("GOWS_MODE"),
		AdminUsername: os.Getenv("GOWS_ADMIN_USERNAME"),
		AdminPassword: os.Getenv("GOWS_ADMIN_PASSWORD"),
	})

	app := &AppConfig{
		AppID:     os.Getenv("APP_ID"),
		TicketURL: os.Getenv("LARAVEL_TICKET_URL"),
		Secret:    os.Getenv("INTERNAL_SECRET"),
	}
	if app.AppID == "" {
		app.AppID = "default"
	}
	if app.TicketURL == "" {
		app.TicketURL = "http://localhost:8000/api/internal/ws/validate-ticket"
	}
	if app.Secret == "" {
		app.Secret = "super-secret-internal-key"
	}
	apps[app.AppID] = app
	hubs[app.AppID] = NewHub()
	infoLog("Loaded single-tenant fallback from env vars (AppID: %s, mode: %s)", app.AppID, serverMode)
}

func applyServerSettings(config ServerConfig) {
	if config.Mode != "" {
		mode := strings.ToLower(strings.TrimSpace(config.Mode))
		if mode == "debug" || mode == "production" {
			serverMode = mode
		} else {
			log.Printf("Invalid server mode %q; using %s", config.Mode, serverMode)
		}
	}
	if config.AdminUsername != "" {
		adminUsername = config.AdminUsername
	}
	if config.AdminPassword != "" {
		adminPassword = config.AdminPassword
	}
}
