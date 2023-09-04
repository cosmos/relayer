package utils

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

func UpdateConfig(filePath string, pathKeys []string, newValue interface{}) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var data map[string]interface{}
	err = yaml.Unmarshal(content, &data)
	if err != nil {
		return err
	}

	currentMap := data
	for i, key := range pathKeys {
		if i == len(pathKeys)-1 {
			currentMap[key] = newValue
		} else {
			nextMap, ok := currentMap[key].(map[string]interface{})
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

	err = os.WriteFile(filePath, updatedContent, 0644)
	if err != nil {
		return err
	}

	return nil
}
