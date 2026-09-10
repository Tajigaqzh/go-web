package config

import (
	"fmt"
	"log"
	"time"

	"github.com/spf13/viper"
)

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
}

func (cfg *DatabaseConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)
}

type ServerConfig struct {
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type LogConfig struct {
	Level    string `mapstructure:"level"`
	Encoding string `mapstructure:"encoding"`
	Dir      string `mapstructure:"dir"`
	Rotate   string `mapstructure:"rotate"`
	MaxAge   int    `mapstructure:"max_age"`
	Console  bool   `mapstructure:"console"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	Secret     string `mapstructure:"secret"`
	AccessTTL  string `mapstructure:"access_ttl"`
	RefreshTTL string `mapstructure:"refresh_ttl"`
}

func (cfg *JWTConfig) AccessDuration() time.Duration {
	d, err := time.ParseDuration(cfg.AccessTTL)
	if err != nil || d <= 0 {
		return 2 * time.Hour
	}
	return d
}

func (cfg *JWTConfig) RefreshDuration() time.Duration {
	d, err := time.ParseDuration(cfg.RefreshTTL)
	if err != nil || d <= 0 {
		return 168 * time.Hour
	}
	return d
}

type Config struct {
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	JWT      JWTConfig      `mapstructure:"jwt"`
	Server   ServerConfig   `mapstructure:"server"`
	Log      LogConfig      `mapstructure:"log"`
}

func LoadConfig() *Config {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	if err := v.ReadInConfig(); err != nil {
		log.Fatalf("failed to read config.yaml: %v (copy config.yaml.example to config.yaml)", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		log.Fatalf("unable to decode config.yaml: %v", err)
	}
	if cfg.JWT.Secret == "" {
		log.Fatal("jwt.secret is required in config.yaml")
	}

	return &cfg
}
