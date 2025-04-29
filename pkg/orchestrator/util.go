package orchestrator

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/rzbill/rune/pkg/types"
)

// generateInstanceName generates a unique instance name for a service
func generateInstanceName(service *types.Service, index int) string {
	return fmt.Sprintf("%s-%d", service.Name, index)
}

func PrettyPrint(v ...interface{}) {
	prints := CapturePrettyPrint(v...)
	fmt.Println(prints...)
}

func CapturePrettyPrint(v ...interface{}) []any {
	prints := []any{}
	for _, value := range v {
		// if v is a string then print it directly
		if str, ok := value.(string); ok {
			prints = append(prints, str)
			continue
		}

		if str, ok := value.(*string); ok {
			prints = append(prints, *str)
			continue
		}

		if intValue, ok := value.(int); ok {
			prints = append(prints, strconv.Itoa(intValue))
			continue
		}

		if intValue, ok := value.(*int); ok {
			prints = append(prints, strconv.Itoa(*intValue))
			continue
		}

		if intValue, ok := value.(int64); ok {
			prints = append(prints, strconv.FormatInt(intValue, 10))
			continue
		}

		if intValue, ok := value.(*int64); ok {
			prints = append(prints, strconv.FormatInt(*intValue, 10))
			continue
		}

		if floatValue, ok := value.(float64); ok {
			prints = append(prints, strconv.FormatFloat(floatValue, 'f', -1, 64))
			continue
		}

		if floatValue, ok := value.(*float64); ok {
			prints = append(prints, strconv.FormatFloat(*floatValue, 'f', -1, 64))
			continue
		}

		// default to pretty print
		prints = append(prints, MapToPretty(value))
	}

	return prints
}

func MapToPretty(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}
	return string(b)
}
