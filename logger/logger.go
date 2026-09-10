package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Log *zap.Logger = zap.NewNop()

type Config struct {
	Level    string `mapstructure:"level"`
	Encoding string `mapstructure:"encoding"`
	Dir      string `mapstructure:"dir"`
	Rotate   string `mapstructure:"rotate"` // day | hour
	MaxAge   int    `mapstructure:"max_age"`
	Console  bool   `mapstructure:"console"`
}

func Init(cfg *Config) error {
	dir := cfg.Dir
	if dir == "" {
		dir = "logs"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	minLevel := zapcore.InfoLevel
	if err := minLevel.UnmarshalText([]byte(cfg.Level)); err != nil {
		minLevel = zapcore.InfoLevel
	}

	rotate := strings.ToLower(cfg.Rotate)
	if rotate != "hour" {
		rotate = "day"
	}
	maxAge := cfg.MaxAge
	if maxAge <= 0 {
		maxAge = 7
	}

	fileEncoder := newEncoder("json")
	consoleEncoder := newEncoder(cfg.Encoding)

	cores := make([]zapcore.Core, 0, 5)
	for _, levelName := range []string{"debug", "info", "warn", "error"} {
		writer, err := newRotateWriter(dir, levelName, rotate, maxAge)
		if err != nil {
			return err
		}
		target := parseLevel(levelName)
		cores = append(cores, zapcore.NewCore(
			fileEncoder,
			zapcore.AddSync(writer),
			levelFilter(minLevel, target),
		))
	}

	if cfg.Console {
		cores = append(cores, zapcore.NewCore(
			consoleEncoder,
			zapcore.AddSync(os.Stdout),
			minLevel,
		))
	}

	Log = zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddCallerSkip(0))
	return nil
}

func Sync() {
	_ = Log.Sync()
}

func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		Log.Info("request",
			zap.Int("status", c.Writer.Status()),
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Duration("latency", time.Since(start)),
			zap.String("client_ip", c.ClientIP()),
		)
	}
}

func newEncoder(encoding string) zapcore.Encoder {
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encCfg.TimeKey = "time"
	encCfg.MessageKey = "msg"
	encCfg.LevelKey = "level"
	encCfg.CallerKey = "caller"
	if strings.EqualFold(encoding, "console") {
		encCfg = zap.NewDevelopmentEncoderConfig()
		encCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encCfg.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05")
		return zapcore.NewConsoleEncoder(encCfg)
	}
	return zapcore.NewJSONEncoder(encCfg)
}

func newRotateWriter(dir, level, rotate string, maxAgeDays int) (zapcore.WriteSyncer, error) {
	var pattern string
	var rotation time.Duration
	if rotate == "hour" {
		pattern = filepath.Join(dir, level+"-%Y-%m-%d-%H.log")
		rotation = time.Hour
	} else {
		pattern = filepath.Join(dir, level+"-%Y-%m-%d.log")
		rotation = 24 * time.Hour
	}

	writer, err := rotatelogs.New(
		pattern,
		rotatelogs.WithMaxAge(time.Duration(maxAgeDays)*24*time.Hour),
		rotatelogs.WithRotationTime(rotation),
	)
	if err != nil {
		return nil, fmt.Errorf("create rotate writer for %s: %w", level, err)
	}
	return zapcore.AddSync(writer), nil
}

func parseLevel(name string) zapcore.Level {
	switch name {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn":
		return zapcore.WarnLevel
	default:
		return zapcore.ErrorLevel
	}
}

func levelFilter(minLevel, target zapcore.Level) zapcore.LevelEnabler {
	return zap.LevelEnablerFunc(func(l zapcore.Level) bool {
		if l < minLevel {
			return false
		}
		if target == zapcore.ErrorLevel {
			return l >= zapcore.ErrorLevel
		}
		return l == target
	})
}
