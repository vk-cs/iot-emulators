package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"time"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/mailru/surgemq/message"
	"github.com/mailru/surgemq/service"
	sdk "github.com/vk-cs/iot-go-agent-sdk"
	"github.com/vk-cs/iot-go-agent-sdk/gen/swagger/http_client/client"
	"github.com/vk-cs/iot-go-agent-sdk/gen/swagger/http_client/client/agents"
	"github.com/vk-cs/iot-go-agent-sdk/gen/swagger/http_client/models"
)

type MQTTEmulator struct {
	logger     *zap.Logger
	source     *DataSource
	agentID    *int64
	cli        *client.HTTP
	mqttClient *service.Client
	timeout    time.Duration
	cfg        AgentConfig
	login      string
	password   string
	host       string
}

func (e *MQTTEmulator) getConfig(ctx context.Context) (*models.ConfigObject, error) {
	requestCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	params := &agents.GetAgentConfigParams{
		// get latest version
		Version: nil,
		Context: requestCtx,
	}
	agentConfig, err := e.cli.Agents.GetAgentConfig(
		params, httptransport.BasicAuth(e.login, e.password),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent config: %w", err)
	}

	return agentConfig.Payload, nil
}

func (e *MQTTEmulator) sendEvents(tags []*models.TagValueObject) error {
	pubmsg := message.NewPublishMessage()
	pubmsg.SetPacketId(uint16(rand.Intn(math.MaxUint16)))
	pubmsg.SetRetain(true)

	if err := pubmsg.SetTopic(e.getTopic()); err != nil {
		return fmt.Errorf("failed to set topic: %w", err)
	}

	if err := pubmsg.SetQoS(message.QosAtLeastOnce); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	body, err := json.Marshal(&models.AddEvent{Tags: tags})
	if err != nil {
		return fmt.Errorf("failed to marshal body: %w", err)
	}
	pubmsg.SetPayload(body)

	err = e.mqttClient.Publish(pubmsg, nil)
	if err != nil {
		return fmt.Errorf("failed to send events: %w", err)
	}

	return nil
}

func (e *MQTTEmulator) getTopic() []byte {
	return []byte(fmt.Sprintf("iot/event/fmt/json"))
}

func (e *MQTTEmulator) connect() error {
	// Creates a new MQTT CONNECT message and sets the proper parameters
	msg := message.NewConnectMessage()
	if err := msg.SetWillQos(message.QosAtLeastOnce); err != nil {
		return fmt.Errorf("failed to set will QoS: %w", err)
	}
	if err := msg.SetVersion(4); err != nil {
		return fmt.Errorf("failed to set version: %w", err)
	}
	if err := msg.SetClientId([]byte(uuid.New().String())); err != nil {
		return fmt.Errorf("failed to set client ID: %w", err)
	}
	msg.SetWillFlag(true)
	msg.SetUsername([]byte(e.login))
	msg.SetPassword([]byte(e.password))
	msg.SetCleanSession(true)

	return e.mqttClient.Connect(e.host, msg)
}

func (e *MQTTEmulator) onStart() error {
	if err := e.connect(); err != nil {
		return fmt.Errorf("failed to connect to broker: %w", err)
	}

	now := sdk.Now()
	err := e.sendEvents([]*models.TagValueObject{
		// set agent status
		{
			ID:        e.cfg.StatusTag.ID,
			Timestamp: &now,
			Value:     sdk.Online,
		},
		// set device status
		{
			ID:        e.cfg.Device.StatusTag.ID,
			Timestamp: &now,
			Value:     sdk.Online,
		},
		// set agent config version
		{
			ID:        e.cfg.ConfigTags.VersionTag.ID,
			Timestamp: &now,
			Value:     e.cfg.Version,
		},
		{
			ID:        e.cfg.ConfigTags.UpdatedAtTag.ID,
			Timestamp: &now,
			Value:     now,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send initial tags: %w", err)
	}

	return nil
}

func (e *MQTTEmulator) onShutdown() error {
	now := sdk.Now()
	err := e.sendEvents([]*models.TagValueObject{
		// set agent status
		{
			ID:        e.cfg.StatusTag.ID,
			Timestamp: &now,
			Value:     sdk.Offline,
		},
		// set device status
		{
			ID:        e.cfg.Device.StatusTag.ID,
			Timestamp: &now,
			Value:     sdk.Offline,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send tags on shutdown: %w", err)
	}

	e.mqttClient.Disconnect()

	return nil
}

func (e *MQTTEmulator) Bootstrap(ctx context.Context) error {
	e.logger.Info("Bootstrapping MQTT emulator")
	rawConfig, err := e.getConfig(ctx)
	if err != nil {
		return err
	}

	e.agentID = rawConfig.Agent.ID

	e.cfg, err = parseConfig(*rawConfig)

	return err
}

func (e *MQTTEmulator) Run(ctx context.Context) error {
	e.logger.Info("Running MQTT emulator")

	err := e.onStart()
	if err != nil {
		return err
	}

	defer func() {
		err := e.onShutdown()
		if err != nil {
			e.logger.Error("Failed to do actions on shutdown", zap.Error(err))
		}
	}()

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		tag := e.cfg.Device.TemperatureTag
		ticker := time.NewTicker(
			getDurationFromDriverConfig(tag.DriverConfig, "event_scrape_interval", time.Minute),
		)
		defer ticker.Stop()
		for {
			select {
			case <-groupCtx.Done():
				return nil

			case <-ticker.C:
				value := e.source.GetTemperatureValue()
				e.logger.Info("Sending temperature value", zap.Float64("value", value))
				now := sdk.Now()
				err := e.sendEvents([]*models.TagValueObject{
					{
						ID:        tag.ID,
						Timestamp: &now,
						Value:     value,
					},
				})
				if err != nil {
					return fmt.Errorf("failed to send temperature value: %w", err)
				}
			}
		}
	})
	group.Go(func() error {
		tag := e.cfg.Device.HumidityTag
		ticker := time.NewTicker(
			getDurationFromDriverConfig(tag.DriverConfig, "event_scrape_interval", time.Minute),
		)
		defer ticker.Stop()
		for {
			select {
			case <-groupCtx.Done():
				return nil

			case <-ticker.C:
				value := e.source.GetHumidityValue()
				e.logger.Info("Sending humidity value", zap.Float64("value", value))
				now := sdk.Now()
				err := e.sendEvents([]*models.TagValueObject{
					{
						ID:        tag.ID,
						Timestamp: &now,
						Value:     value,
					},
				})
				if err != nil {
					return fmt.Errorf("failed to send humidity value: %w", err)
				}
			}
		}
	})
	group.Go(func() error {
		tag := e.cfg.Device.LightTag
		ticker := time.NewTicker(
			getDurationFromDriverConfig(tag.DriverConfig, "state_scrape_interval", time.Minute),
		)
		defer ticker.Stop()
		for {
			select {
			case <-groupCtx.Done():
				return nil

			case <-ticker.C:
				value := e.source.GetLightValue()
				e.logger.Info("Sending light state value", zap.Bool("value", value))
				now := sdk.Now()
				err := e.sendEvents([]*models.TagValueObject{
					{
						ID:        tag.ID,
						Timestamp: &now,
						Value:     value,
					},
				})
				if err != nil {
					return fmt.Errorf("failed to send light value: %w", err)
				}
			}
		}
	})

	return group.Wait()
}

func NewMQTTEmulator(
	logger *zap.Logger, source *DataSource, cli *client.HTTP, timeout time.Duration, login, password, host string,
) *MQTTEmulator {
	return &MQTTEmulator{
		logger:     logger,
		cli:        cli,
		mqttClient: &service.Client{},
		timeout:    timeout,
		source:     source,
		login:      login,
		password:   password,
		host:       host,
	}
}
