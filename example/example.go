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
	if err := chatbotReplier.SimpleReplyText(ctx, data.SessionWebhook, replyMsg); err != nil {
		return nil, err
	}
	if err := chatbotReplier.SimpleReplyMarkdown(ctx, data.SessionWebhook, []byte("Markdown消息"), replyMsg); err != nil {
		return nil, err
	}

	return []byte(""), nil
}

// 简单的插件处理实现
func OnPluginMessageReceived(ctx context.Context, request *plugin.GraphRequest) (*plugin.GraphResponse, error) {
	response := &plugin.GraphResponse{
		Body: `{"text": "hello world", "content": [{"title": "1", "description": "2", "url":"https://www.zhihu.com/question/626551401"},{"title": "2", "description": "2", "url":"https://www.zhihu.com/question/626551401"}]}`,
	}
	return response, nil
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
	if err := frameResp.SetJson(event.NewEventProcessResultSuccess()); err != nil {
		return nil, err
	}

	return
}

// go run example/*.go --client_id your-client-id --client_secret your-client-secret
func main() {
	var clientId, clientSecret string
	flag.StringVar(&clientId, "client_id", "", "your-client-id")
	flag.StringVar(&clientSecret, "client_secret", "", "your-client-secret")

	flag.Parse()

	logger.SetLogger(logger.NewStdTestLoggerWithDebug())

	cli := client.NewStreamClient(client.WithAppCredential(client.NewAppCredentialConfig(clientId, clientSecret)))

	//注册事件类型的处理函数
	cli.RegisterAllEventRouter(OnEventReceived)
	//注册callback类型的处理函数
	cli.RegisterChatBotCallbackRouter(OnChatBotMessageReceived)
	//注册插件的处理函数
	cli.RegisterPluginCallbackRouter(OnPluginMessageReceived)

	err := cli.Start(context.Background())
	if err != nil {
		panic(err)
	}

	defer cli.Close()

	select {}
}
