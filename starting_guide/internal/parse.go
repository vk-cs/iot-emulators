package internal

import (
	"errors"

	"github.com/vk-cs/iot-go-agent-sdk/gen/swagger/http_client/models"
)

type ConfigTags struct {
	VersionTag models.TagConfigObject
	UpdatedAtTag models.TagConfigObject
}

type DeviceConfig struct {
	StatusTag models.TagConfigObject
	TemperatureTag models.TagConfigObject
	HumidityTag models.TagConfigObject
	LightTag models.TagConfigObject
}

type AgentConfig struct {
	StatusTag models.TagConfigObject
	ConfigTags ConfigTags
	Version *string
	Device DeviceConfig
}

// FIXME: move this to sdk
func findTagByPath(tags []*models.TagConfigObject, path []string) (*models.TagConfigObject, bool) {
	for _, tag := range tags {
		if *tag.Name == path[0] {
			if len(path) == 1 {
				return tag, true
			} else {
				return findTagByPath(tag.Children, path[1:])
			}
		}
	}

	return nil, false
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
	agentStatusTag, found := findTagByPath(input.Agent.Tag.Children, []string{"$state", "$status"})
	if !found {
		return AgentConfig{}, errors.New("not found status tag in agent config")
	}

	// FIXME: move system tags paths to sdk
	agentConfigVersionTag, found := findTagByPath(input.Agent.Tag.Children, []string{"$state", "$config", "$version"})
	if !found {
		return AgentConfig{}, errors.New("not found status tag in device config")
	}

	agentConfigUpdatedAtTag, found := findTagByPath(input.Agent.Tag.Children, []string{"$state", "$config", "$updated_at"})
	if !found {
		return AgentConfig{}, errors.New("not found status tag in device config")
	}

	device, found := findDeviceByName(input.Agent.Devices, "emulator")
	if !found {
		return AgentConfig{}, errors.New("not found device in agent config")
	}

	deviceStatusTag, found := findTagByPath(input.Agent.Tag.Children, []string{"$state", "$status"})
	if !found {
		return AgentConfig{}, errors.New("not found status tag in device config")
	}

	deviceTemperatureTag, found := findTagByPath(device.Tag.Children, []string{"temperature"})
	if !found {
		return AgentConfig{}, errors.New("not found temperature tag in device config")
	}

	deviceHumidityTag, found := findTagByPath(device.Tag.Children, []string{"humidity"})
	if !found {
		return AgentConfig{}, errors.New("not found humidity tag in device config")
	}

	deviceLightTag, found := findTagByPath(device.Tag.Children, []string{"light"})
	if !found {
		return AgentConfig{}, errors.New("not found light tag in device config")
	}

	return AgentConfig{
		StatusTag: *agentStatusTag,
		Device: DeviceConfig{
			StatusTag: *deviceStatusTag,
			TemperatureTag: *deviceTemperatureTag,
			HumidityTag: *deviceHumidityTag,
			LightTag: *deviceLightTag,
		},
		ConfigTags: ConfigTags{
			VersionTag: *agentConfigVersionTag,
			UpdatedAtTag: *agentConfigUpdatedAtTag,
		},
		Version: input.Version,
	}, nil
}
