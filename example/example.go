package main

import (
	"context"
	"fmt"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/clientV2"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/event"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
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
	e := clientV2.
		NewBuilder().
		Credential(&clientV2.AuthClientCredential{ClientId: "put your app clientId", ClientSecret: "put your app clientSecret"}).
		//监听机器人回调
		RegisterCallbackHandler("/v1.0/im/bot/messages/get", HandMyBot).
		//开放平台事件
		RegisterAllEventHandler(func(event *clientV2.GenericOpenDingTalkEvent) clientV2.EventStatus {
			println("收到消息", event.EventId)
			println("收到消息", event.EventType)
			println("收到消息", event.EventCorpId)
			println("收到消息", event.EventUnifiedAppId)
			return clientV2.EventStatusSuccess
		}).
		Build().
		Start(context.Background())
	if e != nil {
		println("启动失败", e.Error())
		return
	}

	select {}
}

func HandMyBot(data *chatbot.BotCallbackDataModel) (*chatbot.BotCallbackRespModel, error) {
	replyMsg := []byte(fmt.Sprintf("msg received: [%s]", data.Text.Content))

	chatbotReplier := chatbot.NewChatbotReplier()
	chatbotReplier.SimpleReplyText(context.Background(), data.SessionWebhook, replyMsg)
	chatbotReplier.SimpleReplyMarkdown(context.Background(), data.SessionWebhook, []byte("Markdown消息"), replyMsg)

	return &chatbot.BotCallbackRespModel{}, nil
}
