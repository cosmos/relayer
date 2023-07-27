package common

import "fmt"

func GetIdentifier(name string, i int) string {
	return fmt.Sprintf("%s-%d", name, i)
}
