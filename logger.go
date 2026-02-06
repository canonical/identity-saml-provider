package main

import "go.uber.org/zap"

// zapLoggerAdapter adapts zap logger to the logger interface expected by the SAML library
type zapLoggerAdapter struct {
	sugar *zap.SugaredLogger
}

func (z *zapLoggerAdapter) Print(args ...interface{}) {
	z.sugar.Info(args...)
}

func (z *zapLoggerAdapter) Println(args ...interface{}) {
	z.sugar.Info(args...)
}

func (z *zapLoggerAdapter) Printf(format string, args ...interface{}) {
	z.sugar.Infof(format, args...)
}

func (z *zapLoggerAdapter) Fatal(args ...interface{}) {
	z.sugar.Fatal(args...)
}

func (z *zapLoggerAdapter) Fatalln(args ...interface{}) {
	z.sugar.Fatal(args...)
}

func (z *zapLoggerAdapter) Fatalf(format string, args ...interface{}) {
	z.sugar.Fatalf(format, args...)
}

func (z *zapLoggerAdapter) Panic(args ...interface{}) {
	z.sugar.Panic(args...)
}

func (z *zapLoggerAdapter) Panicln(args ...interface{}) {
	z.sugar.Panic(args...)
}

func (z *zapLoggerAdapter) Panicf(format string, args ...interface{}) {
	z.sugar.Panicf(format, args...)
}

// newZapStdLogger creates a logger adapter that the SAML library can use
func newZapStdLogger(zapLogger *zap.Logger) *zapLoggerAdapter {
	return &zapLoggerAdapter{sugar: zapLogger.Sugar()}
}
