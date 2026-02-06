package provider

import "go.uber.org/zap"

// ZapLoggerAdapter adapts zap logger to the logger interface expected by the SAML library
type ZapLoggerAdapter struct {
	sugar *zap.SugaredLogger
}

func (z *ZapLoggerAdapter) Print(args ...interface{}) {
	z.sugar.Info(args...)
}

func (z *ZapLoggerAdapter) Println(args ...interface{}) {
	z.sugar.Info(args...)
}

func (z *ZapLoggerAdapter) Printf(format string, args ...interface{}) {
	z.sugar.Infof(format, args...)
}

func (z *ZapLoggerAdapter) Fatal(args ...interface{}) {
	z.sugar.Fatal(args...)
}

func (z *ZapLoggerAdapter) Fatalln(args ...interface{}) {
	z.sugar.Fatal(args...)
}

func (z *ZapLoggerAdapter) Fatalf(format string, args ...interface{}) {
	z.sugar.Fatalf(format, args...)
}

func (z *ZapLoggerAdapter) Panic(args ...interface{}) {
	z.sugar.Panic(args...)
}

func (z *ZapLoggerAdapter) Panicln(args ...interface{}) {
	z.sugar.Panic(args...)
}

func (z *ZapLoggerAdapter) Panicf(format string, args ...interface{}) {
	z.sugar.Panicf(format, args...)
}

// NewZapStdLogger creates a logger adapter that the SAML library can use
func NewZapStdLogger(zapLogger *zap.Logger) *ZapLoggerAdapter {
	return &ZapLoggerAdapter{sugar: zapLogger.Sugar()}
}
