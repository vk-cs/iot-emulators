package internal

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"

	"github.com/vk-cs/iot-go-agent-sdk/gen/swagger/http_client/client"
	"github.com/vk-cs/iot-go-agent-sdk/gen/swagger/http_client/client/agents"
	"github.com/vk-cs/iot-go-agent-sdk/gen/swagger/http_client/models"
	"github.com/vk-cs/iot-go-agent-sdk/gen/swagger/http_client/client/events"
)

// FIXME: move this to sdk
func getTimestamp() *int64 {
	now := time.Now().UnixNano() / 1000
	return &now
}

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

	if time.Second * 10 <= d && d <= time.Hour {
		return d
	} else {
		return fallback
	}
}

type Emulator struct {
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
	now := getTimestamp()
	err := e.sendEvents(ctx, []*models.TagValueObject{
		// set agent status
		{
			ID: e.cfg.StatusTag.ID,
			Timestamp: now,
			// FIXME: move status values to sdk
			Value: "online",
		},
		// set device status
		{
			ID: e.cfg.Device.StatusTag.ID,
			Timestamp: now,
			Value: "online",
		},
		// set agent config version
		{
			ID: e.cfg.ConfigTags.VersionTag.ID,
			Timestamp: now,
			Value: e.cfg.Version,
		},
		{
			ID: e.cfg.ConfigTags.UpdatedAtTag.ID,
			Timestamp: now,
			Value: now,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send initial tags: %w", err)
	}

	return nil
}

func (e *Emulator) onShutdown(ctx context.Context) error {
	now := getTimestamp()
	err := e.sendEvents(ctx, []*models.TagValueObject{
		// set agent status
		{
			ID:        e.cfg.StatusTag.ID,
			Timestamp: now,
			Value:     "offline",
		},
		// set device status
		{
			ID:        e.cfg.Device.StatusTag.ID,
			Timestamp: now,
			Value:     "offline",
		},
	})
	if err != nil {
		return fmt.Errorf("failed to send tags on shutdown: %w", err)
	}

	return nil
}

func (e *Emulator) Bootstrap(ctx context.Context) error {
	rawConfig, err := e.getConfig(ctx)
	if err != nil {
		return err
	}

	e.cfg, err = parseConfig(*rawConfig)
	return err
}

func (e *Emulator) Run(ctx context.Context) error {
	err := e.onStart(ctx)
	if err != nil {
		return err
	}

	defer func() {
		err := e.onShutdown(context.Background())
		if err != nil {

		}
	}()

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		tag := e.cfg.Device.TemperatureTag
		ticker := time.NewTicker(getDurationFromDriverConfig(tag.DriverConfig, "event_scrape_interval", time.Minute))
		for {
			select {
			case <-groupCtx.Done():
				return nil

			case <-ticker.C:
				err := e.sendEvents(groupCtx, []*models.TagValueObject{
					{
						ID: tag.ID,
						Timestamp: getTimestamp(),
						Value: e.source.GetTemperatureValue(),
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
		for {
			select {
			case <-groupCtx.Done():
				return nil

			case <-ticker.C:
				err := e.sendEvents(groupCtx, []*models.TagValueObject{
					{
						ID: tag.ID,
						Timestamp: getTimestamp(),
						Value: e.source.GetHumidityValue(),
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
		for {
			select {
			case <-groupCtx.Done():
				return nil

			case <-ticker.C:
				err := e.sendEvents(groupCtx, []*models.TagValueObject{
					{
						ID: tag.ID,
						Timestamp: getTimestamp(),
						Value: e.source.GetLightValue(),
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

func NewEmulator(source *DataSource, cli *client.HTTP, timeout time.Duration, login, password string) *Emulator {
	return &Emulator{
		cli: cli,
		timeout: timeout,
		source: source,
		authInfo: httptransport.BasicAuth(login, password),
	}
}
