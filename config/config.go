package config

import (
	"flag"
	"os"
	"strconv"
)

type Config struct {
	Port      int
	JWTSecret string
	DBPath    string
	DebugMode bool
}

var AppConfig *Config

func Init() {
	// dedicated flag set to avoid interference if tested
	var portFlag int

	// Only define and parse if not already parsed (safeguard)
	if !flag.Parsed() {
		flag.IntVar(&portFlag, "port", 0, "Port to run the server on")
		flag.Parse()
	}

	port := 7002
	if p := os.Getenv("PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil {
			port = parsed
		}
	}

	// Flag overrides env and default if provided (non-zero)
	if portFlag != 0 {
		port = portFlag
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "netcontrol-secret-key-change-in-production"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/netcontrol.db"
	}

	AppConfig = &Config{
		Port:      port,
		JWTSecret: jwtSecret,
		DBPath:    dbPath,
		DebugMode: os.Getenv("DEBUG") == "true",
	}
}

func Get() *Config {
	if AppConfig == nil {
		Init()
	}
	return AppConfig
}
