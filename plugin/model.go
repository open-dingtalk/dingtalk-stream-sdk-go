package plugin

import (
	"errors"
	"fmt"
	"reflect"
)

type PluginMessage struct {
	PluginId      string      `json:"pluginId"`
	PluginVersion string      `json:"pluginVersion"`
	AbilityKey    string      `json:"abilityKey"`
	Data          interface{} `json:"data"`
	RequestId     string      `json:"requestId"`
}

// 用于将数据转换成插件的请求参数
func (req *PluginMessage) ParseData(model interface{}) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = errors.New(fmt.Sprintf("parse data error: %v", e))
		}
	}()
	m, ok := req.Data.(map[string]interface{})
	if !ok {
		return errors.New(fmt.Sprintf("invalid data: %v", req.Data))
	}
	pValue := reflect.ValueOf(model).Elem()
	pType := pValue.Type()
	for i := 0; i < pType.NumField(); i++ {
		field := pType.Field(i)
		if value, ok := m[field.Name]; ok {
			pValue.Field(i).Set(reflect.ValueOf(value))
		}
	}
	return nil
}

type PluginResponse struct {
	Result    interface{} `json:"result"`
	RequestId string      `json:"requestId"`
}
