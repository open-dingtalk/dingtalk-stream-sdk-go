package plugin

import (
	"encoding/json"
)

type PluginMessage struct {
	PluginId      string      `json:"pluginId"`
	PluginVersion string      `json:"pluginVersion"`
	AbilityKey    string      `json:"abilityKey"`
	Data          interface{} `json:"data"`
	RequestId     string      `json:"requestId"`
}

// 用于将数据转换成插件的请求参数
func (req *PluginMessage) ParseRequest(pluginRequest interface{}) error {
	data, err := json.Marshal(req.Data)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, pluginRequest)
	if err != nil {
		return err
	}
	return nil
}

type PluginResponse struct {
	Result    interface{} `json:"result"`
	RequestId string      `json:"requestId"`
}
