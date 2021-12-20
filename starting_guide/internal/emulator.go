package internal

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"time"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"golang.org/x/sync/errgroup"

	sdk "github.com/vk-cs/iot-go-agent-sdk"
	"github.com/vk-cs/iot-go-agent-sdk/gen/swagger/http_client/client"
	"github.com/vk-cs/iot-go-agent-sdk/gen/swagger/http_client/client/agents"
	"github.com/vk-cs/iot-go-agent-sdk/gen/swagger/http_client/client/events"
	"github.com/vk-cs/iot-go-agent-sdk/gen/swagger/http_client/models"
)

// FIXME: move this to sdk
func getDurationFromDriverConfig(input interface{}, key string, fallback time.Duration) time.Duration {
	if input == nil {
		return fallback
	}

	inputMap, ok := input.(map[string]interface{})
	if !ok {
		return fallback
	}

	rawValue, ok := inputMap[key]
	if !ok {
		return fallback
	}

	stringValue, ok := rawValue.(string)
	if !ok {
		return fallback
	}

	d, err := time.ParseDuration(stringValue)
	if err != nil {
		return fallback
	}

	if time.Second*10 <= d && d <= time.Hour {
		return d
	} else {
		return fallback
	}
}

type Emulator struct {
	logger   *zap.Logger
	source   *DataSource
	cli      *client.HTTP
	timeout  time.Duration
	cfg      AgentConfig
	authInfo runtime.ClientAuthInfoWriter
}

func (e *Emulator) getConfig(ctx context.Context) (*models.ConfigObject, error) {
	requestCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	params := &agents.GetAgentConfigParams{
		// get latest version
		Version: nil,
		Context: requestCtx,
	}
	agentConfig, err := e.cli.Agents.GetAgentConfig(params, e.authInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent config: %w", err)
	}

	return agentConfig.Payload, nil
}

func (e *Emulator) sendEvents(ctx context.Context, tags []*models.TagValueObject) error {
	requestCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	params := &events.AddEventParams{
		Context: requestCtx,
		Body: &models.AddEvent{
			Tags: tags,
		},
	}
	_, err := e.cli.Events.AddEvent(params, e.authInfo)
	if err != nil {
		return fmt.Errorf("failed to send events: %w", err)
	}

	return nil
}

func (e *Emulator) onStart(ctx context.Context) error {
	now := sdk.Now()
	err := e.sendEvents(ctx, []*models.TagValueObject{
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

func (e *Emulator) onShutdown(ctx context.Context) error {
	now := sdk.Now()
	err := e.sendEvents(ctx, []*models.TagValueObject{
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

	return nil
}

func (e *Emulator) Bootstrap(ctx context.Context) error {
	e.logger.Info("Bootstrapping emulator")
	rawConfig, err := e.getConfig(ctx)
	if err != nil {
		return err
	}

	e.cfg, err = parseConfig(*rawConfig)
	return err
}

func (e *Emulator) Run(ctx context.Context) error {
	e.logger.Info("Running emulator")

	err := e.onStart(ctx)
	if err != nil {
		return err
	}

	defer func() {
		err := e.onShutdown(context.Background())
		if err != nil {
			e.logger.Error("Failed to do actions on shutdown", zap.Error(err))
		}
	}()

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		tag := e.cfg.Device.TemperatureTag
		ticker := time.NewTicker(getDurationFromDriverConfig(tag.DriverConfig, "event_scrape_interval", time.Minute))
		defer ticker.Stop()
		for {
			select {
			case <-groupCtx.Done():
				return nil

			case <-ticker.C:
				value := e.source.GetTemperatureValue()
				e.logger.Info("Sending temperature value", zap.Float64("value", value))
				now := sdk.Now()
				err := e.sendEvents(groupCtx, []*models.TagValueObject{
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
		ticker := time.NewTicker(getDurationFromDriverConfig(tag.DriverConfig, "event_scrape_interval", time.Minute))
		defer ticker.Stop()
		for {
			select {
			case <-groupCtx.Done():
				return nil

			case <-ticker.C:
				value := e.source.GetHumidityValue()
				e.logger.Info("Sending humidity value", zap.Float64("value", value))
				now := sdk.Now()
				err := e.sendEvents(groupCtx, []*models.TagValueObject{
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
		ticker := time.NewTicker(getDurationFromDriverConfig(tag.DriverConfig, "state_scrape_interval", time.Minute))
		defer ticker.Stop()
		for {
			select {
			case <-groupCtx.Done():
				return nil

			case <-ticker.C:
				value := e.source.GetLightValue()
				e.logger.Info("Sending light state value", zap.Bool("value", value))
				now := sdk.Now()
				err := e.sendEvents(groupCtx, []*models.TagValueObject{
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

func NewEmulator(logger *zap.Logger, source *DataSource, cli *client.HTTP, timeout time.Duration, login, password string) *Emulator {
	return &Emulator{
		logger:   logger,
		cli:      cli,
		timeout:  timeout,
		source:   source,
		authInfo: httptransport.BasicAuth(login, password),
	}
}
