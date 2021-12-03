package main

import (
	"context"
	"flag"
	"fmt"
	"go.uber.org/zap/zapcore"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	sdk "github.com/vk-cs/iot-go-agent-sdk"
	"go.uber.org/zap"

	"github.com/vk-cs/iot-emulators/starting_guide/internal"
)

const (
	defaultTimeout = time.Second * 10
)

func main() {
	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:      true,
		Encoding:         "console",
		EncoderConfig:    zapcore.EncoderConfig{
			// Keys can be anything except the empty string.
			TimeKey:        "T",
			LevelKey:       "L",
			NameKey:        "N",
			CallerKey:      "",
			FunctionKey:    zapcore.OmitKey,
			MessageKey:     "M",
			StacktraceKey:  "",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.CapitalLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.StringDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
	logger, err := cfg.Build()
	if err != nil {
		log.Fatalf("Failed to setup logging: %s\n", err.Error())
	}

	login := flag.String("login", "", "login for agent")
	password := flag.String("password", "", "password for agent")

	flag.Parse()
	if *login == ""  {
		fmt.Println("Missing login param")
		os.Exit(1)
	}

	if *password == "" {
		fmt.Println("Missing password param")
		os.Exit(1)
	}

	source, err := internal.NewDataSource()
	if err != nil {
		logger.Fatal("Failed to setup source", zap.Error(err))
	}

	cli := sdk.NewHTTPClient()

	ctx, cancel := context.WithCancel(context.Background())
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		logger.Info("Got termination signal, closing app...")
		cancel()
	}()

	emulator := internal.NewEmulator(source, cli, defaultTimeout, *login, *password)
	err = emulator.Bootstrap(ctx)
	if err != nil {
		logger.Fatal("Failed to bootstrap emulator", zap.Error(err))
	}

	err = emulator.Run(ctx)
	if err != nil {
		logger.Error("Runtime error", zap.Error(err))
	}
}