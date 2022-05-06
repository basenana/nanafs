package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

var (
	logger *zap.Logger
	root   *zap.SugaredLogger
	atom   zap.AtomicLevel
)

func InitLogger() {
	atom = zap.NewAtomicLevel()
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.RFC3339TimeEncoder

	logger = zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderCfg),
		zapcore.Lock(os.Stdout),
		atom,
	))
	root = logger.Sugar()
}

func Sync() {
	_ = logger.Sync()
}

func NewLogger(name string) *zap.SugaredLogger {
	return root.Named(name)
}

func SetDebug(enable bool) {
	if enable {
		atom.SetLevel(zap.DebugLevel)
		return
	}
	atom.SetLevel(zap.InfoLevel)
}
