package clientV2

import (
	"context"
	"errors"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/client"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/utils"
)

type OpenDingTalkClient interface {
	Start(ctx context.Context) error
}

type OpenDingTalkStreamClient struct {
	client *client.StreamClient
}

func (c *OpenDingTalkStreamClient) Start(ctx context.Context) error {
	return c.client.Start(ctx)
}

func NewBuilder() *OpenDingTalkStreamClientBuilder {
	return &OpenDingTalkStreamClientBuilder{callbackSubscription: make(map[string]interface{})}
}

type AuthClientCredential struct {
	ClientId     string
	ClientSecret string
}

type OpenDingTalkStreamClientBuilder struct {
	openApiHost          string
	credential           *AuthClientCredential
	eventHandler         GenericEventHandler
	callbackSubscription map[string]interface{}
}

func (b *OpenDingTalkStreamClientBuilder) PreEnv() *OpenDingTalkStreamClientBuilder {
	b.openApiHost = "https://pre-api.dingtalk.com"
	return b
}

func (b *OpenDingTalkStreamClientBuilder) Credential(credential *AuthClientCredential) *OpenDingTalkStreamClientBuilder {
	b.credential = credential
	return b
}

func (b *OpenDingTalkStreamClientBuilder) RegisterAllEventHandler(h GenericEventHandler) *OpenDingTalkStreamClientBuilder {
	b.eventHandler = h
	return b
}

func (b *OpenDingTalkStreamClientBuilder) RegisterCallbackHandler(topic string, h interface{}) *OpenDingTalkStreamClientBuilder {
	b.callbackSubscription[topic] = h
	return b
}

func (b *OpenDingTalkStreamClientBuilder) Build() OpenDingTalkClient {
	if b.credential == nil {
		panic(errors.New("credential can not empty"))
	}

	options := make([]client.ClientOption, 1)
	options = append(options, client.WithAppCredential(&client.AppCredentialConfig{ClientId: b.credential.ClientId, ClientSecret: b.credential.ClientSecret}))

	if len(b.openApiHost) != 0 {
		options = append(options, client.WithHost(b.openApiHost))
	}
	if b.eventHandler != nil {
		options = append(options, client.WithSubscription(utils.SubscriptionTypeKEvent, "*", EventFacade(b.eventHandler)))
	}
	if len(b.callbackSubscription) != 0 {
		for k, v := range b.callbackSubscription {
			options = append(options, client.WithSubscription(utils.SubscriptionTypeKCallback, k, CallbackFacade(v)))
		}
	}
	c := client.NewStreamClient(options...)
	return &OpenDingTalkStreamClient{client: c}
}
