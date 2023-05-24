package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/event"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
	"net/http"
	"time"
)

/**
 * @Author linya.jj
 * @Date 2023/3/22 18:30
 */

func OnChatBotMessageReceived(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
	frameResp := &payload.DataFrameResponse{
		Code: 200,
		Headers: payload.DataFrameHeader{
			payload.DataFrameHeaderKContentType: payload.DataFrameContentTypeKJson,
			payload.DataFrameHeaderKMessageId:   df.GetMessageId(),
		},
		Message: "ok",
		Data:    "",
	}

	// 反序列化机器人回调消息
	msgData := &chatbot.BotCallbackDataModel{}
	err := json.Unmarshal([]byte(df.Data), msgData)
	if err != nil {
		// TODO 处理错误：回调消息反序列化出错
		return frameResp, nil
	}

	//处理方式：回复文本消息，通过SessionWebhook来回复消息
	requestBody := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]interface{}{
			"content": fmt.Sprintf("msg received: [%s]", msgData.Text.Content),
		},
	}

	requestJsonBody, _ := json.Marshal(requestBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, msgData.SessionWebhook, bytes.NewReader(requestJsonBody))
	if err != nil {
		// TODO 处理错误
		return frameResp, nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")

	httpClient := &http.Client{
		Transport: http.DefaultTransport,
		Timeout:   5 * time.Second, //设置超时，包含connection时间、任意重定向时间、读取response body时间
	}

	_, err = httpClient.Do(req)
	if err != nil {
		// TODO 处理错误: 回复消息出错
		return frameResp, nil
	}

	return frameResp, nil
}

// 事件处理函数
func OnEventReceived(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
	eventHeader := event.NewEventHeaderFromDataFrame(df)

	logger.GetLogger().Infof("received event, eventId=[%s] eventBornTime=[%d] eventCorpId=[%s] eventType=[%s] eventUnifiedAppId=[%s] data=[%s]",
		eventHeader.EventId,
		eventHeader.EventBornTime,
		eventHeader.EventCorpId,
		eventHeader.EventType,
		eventHeader.EventUnifiedAppId,
		df.Data)

	resultStr, _ := json.Marshal(event.NewEventProcessResultSuccess())

	frameResp := &payload.DataFrameResponse{
		Code: payload.DataFrameResponseStatusCodeKOK,
		Headers: payload.DataFrameHeader{
			payload.DataFrameHeaderKContentType: payload.DataFrameContentTypeKJson,
			payload.DataFrameHeaderKMessageId:   df.GetMessageId(),
		},
		Message: "ok",
		Data:    string(resultStr),
	}

	return frameResp, nil
}

func main() {
	var clientId, clientSecret string
	flag.StringVar(&clientId, "client_id", "", "your-client-id")
	flag.StringVar(&clientSecret, "client_secret", "", "your-client-secret")

	flag.Parse()

	logger.SetLogger(logger.NewStdTestLogger())

	cli := client.NewStreamClient(
		client.WithAppCredential(client.NewAppCredentialConfig(clientId, clientSecret)),
		client.WithUserAgent(client.NewDingtalkGoSDKUserAgent()),
	)

	//注册事件类型的处理函数
	cli.RegisterEventRouter("*", OnEventReceived)
	//注册callback类型的处理函数
	cli.RegisterCallbackRouter(payload.BotMessageCallbackTopic, OnChatBotMessageReceived)

	err := cli.Start(context.Background())
	if err != nil {
		panic(err)
	}

	defer cli.Close()

	select {}
}
