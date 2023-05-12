package main

import (
	"context"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/event"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/utils"
)

/**
 * @Author linya.jj
 * @Date 2023/3/22 18:30
 */

func RunEventListener() {
	logger.SetLogger(logger.NewStdTestLogger())

	eventHandler := event.NewDefaultEventFrameHandler(event.EventHandlerDoNothing)

	cli := client.NewStreamClient(
		client.WithAppCredential(client.NewAppCredentialConfig("your-client-id", "your-client-secret")),
		client.WithUserAgent(client.NewDingtalkGoSDKUserAgent()),
		client.WithSubscription(utils.SubscriptionTypeKEvent, "*", eventHandler.OnEventReceived),
	)

	err := cli.Start(context.Background())
	if err != nil {
		panic(err)
	}

	defer cli.Close()

	select {}
}
