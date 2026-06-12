package designcore

import (
	"encoding/json"
	"errors"
)

func jsonUnmarshalObject(raw []byte, target *map[string]any) error {
	if err := json.Unmarshal(raw, target); err != nil {
		return err
	}
	if *target == nil {
		return errors.New("value must be a JSON object")
	}
	return nil
}
