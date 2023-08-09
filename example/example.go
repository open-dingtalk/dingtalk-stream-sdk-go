package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/event"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/plugin"
)

/**
 * @Author linya.jj
 * @Date 2023/3/22 18:30
 */

// 简单的应答机器人实现
func OnChatBotMessageReceived(ctx context.Context, data *chatbot.BotCallbackDataModel) ([]byte, error) {
	replyMsg := []byte(fmt.Sprintf("msg received: [%s]", data.Text.Content))

	chatbotReplier := chatbot.NewChatbotReplier()
	chatbotReplier.SimpleReplyText(ctx, data.SessionWebhook, replyMsg)
	chatbotReplier.SimpleReplyMarkdown(ctx, data.SessionWebhook, []byte("Markdown消息"), replyMsg)

	return []byte(""), nil
}

// 简单的插件处理实现
func OnPluginRequestReceived(ctx context.Context, message *plugin.DingTalkPluginMessage) (interface{}, error) {
	if message.AbilityKey == "echo" {
		echoRequest := &EchoRequest{}
		err := message.ParseData(echoRequest)
		if err != nil {
			return nil, err
		}
		echoResponse := Echo(echoRequest)
		return echoResponse, nil
	}
	return nil, nil
}

// 事件处理
func OnEventReceived(ctx context.Context, df *payload.DataFrame) (frameResp *payload.DataFrameResponse, err error) {
	eventHeader := event.NewEventHeaderFromDataFrame(df)

	logger.GetLogger().Infof("received event, eventId=[%s] eventBornTime=[%d] eventCorpId=[%s] eventType=[%s] eventUnifiedAppId=[%s] data=[%s]",
		eventHeader.EventId,
		eventHeader.EventBornTime,
		eventHeader.EventCorpId,
		eventHeader.EventType,
		eventHeader.EventUnifiedAppId,
		df.Data)

	frameResp = payload.NewSuccessDataFrameResponse()
	frameResp.SetJson(event.NewEventProcessResultSuccess())

	return
}

// go run example/*.go --client_id your-client-id --client_secret your-client-secret
func main() {
	var clientId, clientSecret string
	flag.StringVar(&clientId, "client_id", "", "your-client-id")
	flag.StringVar(&clientSecret, "client_secret", "", "your-client-secret")

	flag.Parse()

	logger.SetLogger(logger.NewStdTestLogger())

	cli := client.NewStreamClient(client.WithAppCredential(client.NewAppCredentialConfig(clientId, clientSecret)))

	//注册事件类型的处理函数
	cli.RegisterAllEventRouter(OnEventReceived)
	//注册callback类型的处理函数
	cli.RegisterChatBotCallbackRouter(OnChatBotMessageReceived)

	cli.RegisterPluginCallbackRouter(OnPluginRequestReceived)
	err := cli.Start(context.Background())
	if err != nil {
		panic(err)
	}

	defer cli.Close()

	select {}
}
