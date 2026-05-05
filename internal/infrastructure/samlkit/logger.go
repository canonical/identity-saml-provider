package samlkit

import "go.uber.org/zap"

// ZapLoggerAdapter adapts a *zap.SugaredLogger to the standard library logger
// interface expected by the crewjam/saml library. It implements Print, Println,
// Printf, Fatal, Fatalln, Fatalf, Panic, Panicln, and Panicf.
type ZapLoggerAdapter struct {
	sugar *zap.SugaredLogger
}

// NewZapLoggerAdapter creates a new ZapLoggerAdapter from a *zap.Logger.
func NewZapLoggerAdapter(zapLogger *zap.Logger) *ZapLoggerAdapter {
	return &ZapLoggerAdapter{sugar: zapLogger.Sugar()}
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
