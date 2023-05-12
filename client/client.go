package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/handler"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/utils"
	"io"
	"net/http"
	"sync"
	"time"
)

/**
 * @Author linya.jj
 * @Date 2023/3/22 14:23
 */

type StreamClient struct {
	AppCredential *AppCredentialConfig
	UserAgent     *UserAgentConfig
	AutoReconnect bool

	subscriptions map[string]map[string]handler.IFrameHandler

	conn      *websocket.Conn
	sessionId string
	mutex     sync.Mutex
}

func NewStreamClient(options ...ClientOption) *StreamClient {
	cli := &StreamClient{}

	defaultOptions := []ClientOption{
		WithSubscription(utils.SubscriptionTypeKSystem, "disconnect", cli.OnDisconnect),
		WithSubscription(utils.SubscriptionTypeKSystem, "ping", cli.OnPing),
		WithUserAgent(NewDingtalkGoSDKUserAgent()),
		WithAutoReconnect(true),
	}

	for _, option := range defaultOptions {
		option(cli)
	}

	for _, option := range options {
		if option == nil {
			continue
		}

		option(cli)
	}

	return cli
}

func (cli *StreamClient) Start(ctx context.Context) error {
	if cli.conn != nil {
		return nil
	}

	cli.mutex.Lock()
	defer cli.mutex.Unlock()

	if cli.conn != nil {
		return nil
	}

	endpoint, err := cli.GetConnectionEndpoint(ctx)
	if err != nil {
		return err
	}

	wssUrl := fmt.Sprintf("%s?ticket=%s", endpoint.Endpoint, endpoint.Ticket)

	header := make(http.Header)

	conn, resp, err := websocket.DefaultDialer.Dial(wssUrl, header)
	if err != nil {
		return err
	}

	// 建连失败
	if resp.StatusCode >= http.StatusBadRequest {
		return utils.ErrorFromHttpResponseBody(resp)
	}

	cli.conn = conn
	cli.sessionId = endpoint.Ticket

	logger.GetLogger().Infof("connect success, sessionId=[%s]", cli.sessionId)

	go cli.processLoop()

	return nil
}

func (cli *StreamClient) processLoop() {
	defer func() {
		if err := recover(); err != nil {
			logger.GetLogger().Errorf("connection process panic due to unknown reason, error=[%s]", err)
		}
		if cli.AutoReconnect {
			go cli.reconnect()
		}
	}()

	for {
		if cli.conn == nil {
			logger.GetLogger().Errorf("connection process connect nil, maybe disconnected.")
			return
		}

		messageType, message, err := cli.conn.ReadMessage()
		if err != nil {
			logger.GetLogger().Errorf("connection process read message error: messageType=[%d] message=[%s] error=[%s]", messageType, string(message), err)
			return
		}

		logger.GetLogger().Debugf("ReadRawMessage : messageType=[%d] message=[%s]", messageType, string(message))

		go cli.processDataFrame(message)
	}
}

func (cli *StreamClient) processDataFrame(rawData []byte) {
	defer func() {
		if err := recover(); err != nil {
			logger.GetLogger().Errorf("connection processDataFrame panic, error=[%s]", err)
		}
	}()

	dataFrame, err := payload.DecodeDataFrame(rawData)
	if err != nil {
		logger.GetLogger().Errorf("connection process decode data frame error: length=[%d] error=[%s]", len(rawData), err)
		return
	}

	if dataFrame == nil || dataFrame.Headers == nil {
		logger.GetLogger().Errorf("connection processDataFrame dataFrame nil.")
		return
	}

	frameHandler, err := cli.GetHandler(dataFrame.Type, dataFrame.GetTopic())
	if err != nil {
		logger.GetLogger().Errorf("connection processDataFrame unregistered handler: type=[%s] topic=[%s]", dataFrame.Type, dataFrame.GetTopic())
		return
	}

	dataAck, err := frameHandler(context.Background(), dataFrame)

	if dataAck == nil && err != nil {
		dataAck = payload.NewErrorDataFrameResponse(dataFrame.GetMessageId(), err)
	}

	if dataAck == nil {
		return
	}

	errSend := cli.SendDataFrameResponse(context.Background(), dataAck)
	logger.GetLogger().Debugf("SendFrameAck dataAck=[%v", dataAck)

	if errSend != nil {
		logger.GetLogger().Errorf("connection processDataFrame send response error: error=[%s]", errSend)
	}
}

func (cli *StreamClient) Close() {
	if cli.conn == nil {
		return
	}

	cli.mutex.Lock()
	defer cli.mutex.Unlock()

	if cli.conn == nil {
		return
	}

	if err := cli.conn.Close(); err != nil {
		logger.GetLogger().Errorf("StreamClient close. error=[%s]", err)
	}
	cli.conn = nil
	cli.sessionId = ""
}

func (cli *StreamClient) reconnect() {
	defer func() {
		if err := recover(); err != nil {
			logger.GetLogger().Errorf("reconect panic due to unknown reason. error=[%s]", err)
		}
	}()

	cli.Close()

	for {
		err := cli.Start(context.Background())
		if err != nil {
			logger.GetLogger().Errorf("StreamClient reconnect error. error=[%s]", err)
			time.Sleep(time.Second * 3)
		} else {
			logger.GetLogger().Infof("StreamClient reconnect success")
			return
		}
	}

}

func (cli *StreamClient) GetHandler(stype, stopic string) (handler.IFrameHandler, error) {
	subs := cli.subscriptions[stype]
	if subs == nil || subs[stopic] == nil {
		return nil, errors.New("HandlerNotRegistedForTypeTopic_" + stype + "_" + stopic)
	}

	return subs[stopic], nil
}

func (cli *StreamClient) CheckConfigValid() error {
	if err := cli.AppCredential.Valid(); err != nil {
		return err
	}

	if err := cli.UserAgent.Valid(); err != nil {
		return err
	}

	if cli.subscriptions == nil {
		return errors.New("subscriptionsNil")
	}

	for ttype, subs := range cli.subscriptions {
		if _, ok := utils.SubscriptionTypeSet[ttype]; !ok {
			return errors.New("UnKnownSubscriptionType_" + ttype)
		}

		if len(subs) <= 0 {
			return errors.New("NoHandlersRegistedForType_" + ttype)
		}

		for ttopic, h := range subs {
			if h == nil {
				return errors.New("HandlerNilForTypeTopic_" + ttype + "_" + ttopic)
			}
		}
	}

	return nil
}

func (cli *StreamClient) GetConnectionEndpoint(ctx context.Context) (*payload.ConnectionEndpointResponse, error) {
	if err := cli.CheckConfigValid(); err != nil {
		return nil, err
	}

	requestModel := payload.ConnectionEndpointRequest{
		ClientId:      cli.AppCredential.ClientId,
		ClientSecret:  cli.AppCredential.ClientSecret,
		UserAgent:     cli.UserAgent.UserAgent,
		Subscriptions: make([]*payload.SubscriptionModel, 0),
	}

	for ttype, subs := range cli.subscriptions {
		for ttopic, _ := range subs {
			requestModel.Subscriptions = append(requestModel.Subscriptions, &payload.SubscriptionModel{
				Type:  ttype,
				Topic: ttopic,
			})
		}
	}

	requestJsonBody, _ := json.Marshal(requestModel)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, utils.GetConnectionEndpointAPIUrl, bytes.NewReader(requestJsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	httpClient := &http.Client{
		Transport: http.DefaultTransport,
		Timeout:   5 * time.Second, //设置超时，包含connection时间、任意重定向时间、读取response body时间
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, utils.ErrorFromHttpResponseBody(resp)
	}

	defer resp.Body.Close()

	responseJsonBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	endpoint := &payload.ConnectionEndpointResponse{}

	if err := json.Unmarshal(responseJsonBody, endpoint); err != nil {
		return nil, err
	}

	if err := endpoint.Valid(); err != nil {
		return nil, err
	}

	return endpoint, nil
}

func (cli *StreamClient) OnDisconnect(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
	logger.GetLogger().Debugf("StreamClient.OnDisconnect")

	cli.Close()
	return nil, nil
}

func (cli *StreamClient) OnPing(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
	dfPong := payload.NewDataFrameAckPong(df.GetMessageId())
	dfPong.Data = df.Data

	return dfPong, nil
}

// 返回正常数据包
func (cli *StreamClient) SendDataFrameResponse(ctx context.Context, resp *payload.DataFrameResponse) error {
	if resp == nil {
		return errors.New("SendDataFrameResponseError_ResponseNil")
	}

	if cli.conn == nil {
		logger.GetLogger().Errorf("SendDataFrameResponse error, conn nil, maybe disconnected.")
		return errors.New("disconnected")
	}
	return cli.conn.WriteJSON(resp)
}
