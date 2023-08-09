package plugin

import (
	"errors"
	"fmt"
	"reflect"
)

type DingTalkPluginMessage struct {
	PluginId      string      `json:"pluginId"`
	PluginVersion string      `json:"pluginVersion"`
	AbilityKey    string      `json:"abilityKey"`
	Data          interface{} `json:"data"`
	RequestId     string      `json:"requestId"`
}

func (req *DingTalkPluginMessage) ParseData(model interface{}) error {
	//TO DO 处理异常
	defer func() {
		recover()
	}()
	m, ok := req.Data.(map[string]interface{})
	if !ok {
		return errors.New(fmt.Sprintf("invalid data: %v", req.Data))
	}
	stValue := reflect.ValueOf(model).Elem()
	sType := stValue.Type()
	for i := 0; i < sType.NumField(); i++ {
		field := sType.Field(i)
		if value, ok := m[field.Name]; ok {
			stValue.Field(i).Set(reflect.ValueOf(value))
		}
	}
	return nil
}

type DingTalkPluginResponse struct {
	Result    interface{} `json:"result"`
	RequestId string      `json:"requestId"`
}
