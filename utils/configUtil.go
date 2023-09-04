package utils

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

const (
	//Add more consts when configUtil is used for other updates
	Chains = "chains"
	Value  = "value"
	Key    = "key"
)

// convertMap recursively converts map[interface{}]interface{} to map[string]interface{}
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

func UpdateConfig(configPath string, pathKeys []string, newValue interface{}) error {
	content, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var rawData map[interface{}]interface{}
	err = yaml.Unmarshal(content, &rawData)
	if err != nil {
		return err
	}
	data := convertMap(rawData)

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
		return err
	}

	err = os.WriteFile(configPath, updatedContent, 0644)
	if err != nil {
		return err
	}

	return nil
}
