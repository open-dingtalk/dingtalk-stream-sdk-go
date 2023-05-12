package main

import (
	"context"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/utils"
)

/**
 * @Author linya.jj
 * @Date 2023/3/22 18:30
 */

func OnBotCallback(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
	frameResp := &payload.DataFrameResponse{
		Code: 200,
		Headers: payload.DataFrameHeader{
			payload.DataFrameHeaderKContentType: payload.DataFrameContentTypeKJson,
			payload.DataFrameHeaderKMessageId:   df.GetMessageId(),
		},
		Message: "ok",
		Data:    "",
	}

	return frameResp, nil
}

func RunBotListener() {
	logger.SetLogger(logger.NewStdTestLogger())

	cli := client.NewStreamClient(
		client.WithAppCredential(client.NewAppCredentialConfig("your-client-id", "your-client-secret")),
		client.WithUserAgent(client.NewDingtalkGoSDKUserAgent()),
		client.WithSubscription(utils.SubscriptionTypeKCallback, payload.BotMessageCallbackTopic, OnBotCallback),
	)

	err := cli.Start(context.Background())
	if err != nil {
		panic(err)
	}

	defer cli.Close()

	select {}
}
