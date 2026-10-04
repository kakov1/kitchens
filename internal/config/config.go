package config

import (
	"log"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env    string `env:"ENV" env-default:"local"`
	Server ServerConfig
	DB     DatabaseConfig
	Redis  RedisConfig
}

type ServerConfig struct {
	Port string `env:"PORT" env-default:"8000"`
}

type DatabaseConfig struct {
	ConnString string `env:"DB_CONN,required"`
}

type RedisConfig struct {
	Addr string `env:"REDIS_ADDR" env-default:"localhost:6379"`
}

func MustLoad() *Config {
	var cfg Config

	if _, err := os.Stat(".env"); err == nil {
		err := cleanenv.ReadConfig(".env", &cfg)
		if err != nil {
			log.Fatalf("failed to read .env file: %s", err)
		}
	} else {
		err := cleanenv.ReadEnv(&cfg)
		if err != nil {
			log.Fatalf("failed to read env variables: %s", err)
		}
	}

	return &cfg
}
