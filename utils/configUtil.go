package utils

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// Constants used in the config utility.Add more as required.
const (
	Chains = "chains"
	Value  = "value"
	Key    = "key"
)

// convertMap recursively converts map[interface{}]interface{} to map[string]interface{}.
func convertMap(input map[interface{}]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range input {
		strKey, ok := key.(string)
		if !ok {
			continue
		}
		if nestedMap, ok := value.(map[interface{}]interface{}); ok {
			result[strKey] = convertMap(nestedMap)
		} else {
			result[strKey] = value
		}
	}
	return result
}

// ReadConfig reads and parses a YAML configuration file.
func ReadConfig(configPath string) (map[string]interface{}, error) {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var rawData map[interface{}]interface{}
	err = yaml.Unmarshal(content, &rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}
	return convertMap(rawData), nil
}

// GetValueFromPath retrieves a value from the configuration using a given path.
func GetValueFromPath(configPath string, path []string) (interface{}, bool) {
	data, err := ReadConfig(configPath)
	if err != nil {
		return nil, false
	}

	current := interface{}(data)
	for _, key := range path {
		asMap, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}

		current, ok = asMap[key]
		if !ok {
			return nil, false
		}
	}

	return current, true
}

// UpdateConfig updates a value in the configuration using a given path.
func UpdateConfig(configPath string, pathKeys []string, newValue interface{}) error {
	data, err := ReadConfig(configPath)
	if err != nil {
		return err
	}

	currentMap := data
	for i, key := range pathKeys {
		value, exists := currentMap[key]
		if !exists {
			return fmt.Errorf("key %s does not exist", key)
		}

		if i == len(pathKeys)-1 {
			currentMap[key] = newValue
		} else {
			nextMap, ok := value.(map[string]interface{})
			if !ok {
				return fmt.Errorf("%s does not lead to a nested map", key)
			}
			currentMap = nextMap
		}
	}

	updatedContent, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal updated data: %w", err)
	}

	err = os.WriteFile(configPath, updatedContent, 0644)
	if err != nil {
		return fmt.Errorf("failed to write updated config: %w", err)
	}

	return nil
}
