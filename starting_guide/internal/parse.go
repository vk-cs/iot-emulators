package internal

import (
	"errors"
	sdk "github.com/vk-cs/iot-go-agent-sdk"

	"github.com/vk-cs/iot-go-agent-sdk/gen/swagger/http_client/models"
)

type ConfigTags struct {
	VersionTag   models.TagConfigObject
	UpdatedAtTag models.TagConfigObject
}

type DeviceConfig struct {
	StatusTag      models.TagConfigObject
	TemperatureTag models.TagConfigObject
	HumidityTag    models.TagConfigObject
	LightTag       models.TagConfigObject
}

type AgentConfig struct {
	StatusTag  models.TagConfigObject
	ConfigTags ConfigTags
	Version    *string
	Device     DeviceConfig
}

func findDeviceByName(input []*models.DeviceConfigObject, name string) (*models.DeviceConfigObject, bool) {
	for _, device := range input {
		if *device.Name == name {
			return device, true
		}
	}

	return nil, false
}

func parseConfig(input models.ConfigObject) (AgentConfig, error) {
	agentStatusTag, found := sdk.FindTagByPath(input.Agent.Tag.Children, sdk.StatusTagPath)
	if !found {
		return AgentConfig{}, errors.New("not found status tag in agent config")
	}

	// FIXME: move system tags paths to sdk
	agentConfigVersionTag, found := sdk.FindTagByPath(input.Agent.Tag.Children, sdk.ConfigVersionTagPath)
	if !found {
		return AgentConfig{}, errors.New("not found config version tag in agent config")
	}

	agentConfigUpdatedAtTag, found := sdk.FindTagByPath(input.Agent.Tag.Children, sdk.ConfigUpdatedAtTagPath)
	if !found {
		return AgentConfig{}, errors.New("not found config updated at tag in agent config")
	}

	device, found := findDeviceByName(input.Agent.Devices, "emulator")
	if !found {
		return AgentConfig{}, errors.New("not found device in agent config")
	}

	deviceStatusTag, found := sdk.FindTagByPath(input.Agent.Tag.Children, sdk.StatusTagPath)
	if !found {
		return AgentConfig{}, errors.New("not found status tag in device config")
	}

	deviceTemperatureTag, found := sdk.FindTagByPath(device.Tag.Children, []string{"temperature"})
	if !found {
		return AgentConfig{}, errors.New("not found temperature tag in device config")
	}

	deviceHumidityTag, found := sdk.FindTagByPath(device.Tag.Children, []string{"humidity"})
	if !found {
		return AgentConfig{}, errors.New("not found humidity tag in device config")
	}

	deviceLightTag, found := sdk.FindTagByPath(device.Tag.Children, []string{"light"})
	if !found {
		return AgentConfig{}, errors.New("not found light tag in device config")
	}

	return AgentConfig{
		StatusTag: *agentStatusTag,
		Device: DeviceConfig{
			StatusTag:      *deviceStatusTag,
			TemperatureTag: *deviceTemperatureTag,
			HumidityTag:    *deviceHumidityTag,
			LightTag:       *deviceLightTag,
		},
		ConfigTags: ConfigTags{
			VersionTag:   *agentConfigVersionTag,
			UpdatedAtTag: *agentConfigUpdatedAtTag,
		},
		Version: input.Version,
	}, nil
}
