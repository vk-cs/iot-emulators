package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	sdk "github.com/vk-cs/iot-go-agent-sdk"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/vk-cs/iot-emulators/starting_guide/internal"
)

const (
	defaultTimeout  = time.Second * 10
	HTTPProtocol    = "http"
	MQTTProtocol    = "mqtt"
	DefaultMQTTHost = "tcp://mqtt-api-iot.mcs.mail.ru:1883"
)

func main() {
	cfg := zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.DebugLevel),
		Development: true,
		Encoding:    "console",
		EncoderConfig: zapcore.EncoderConfig{
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
	protocol := flag.String("protocol", HTTPProtocol, "protocol of agent: mqtt or http")
	mqttHost := flag.String("mqtthost", DefaultMQTTHost, "mqtt host with `tcp://{host}:{port}` format")

	flag.Parse()
	if *login == "" {
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

	var emulator internal.Emulator

	switch *protocol {
	case HTTPProtocol:
		emulator = internal.NewHTTPEmulator(logger, source, cli, defaultTimeout, *login, *password)
	case MQTTProtocol:
		emulator = internal.NewMQTTEmulator(logger, source, cli, defaultTimeout, *login, *password, *mqttHost)
	default:
		{
			fmt.Println("Incorrect protocol param:", *protocol)
			os.Exit(1)
		}
	}

	err = emulator.Bootstrap(ctx)
	if err != nil {
		logger.Fatal("Failed to bootstrap emulator", zap.Error(err))
	}

	err = emulator.Run(ctx)
	if err != nil {
		logger.Error("Runtime error", zap.Error(err))
	}
}
