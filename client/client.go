package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/card"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/handler"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/logger"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/plugin"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/utils"
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

	conn          *websocket.Conn
	sessionId     string
	mutex         sync.Mutex
	writeMutex    sync.Mutex
	startMutex    sync.Mutex
	lifecycleCtx  context.Context
	cancel        context.CancelFunc
	extras        map[string]string
	openApiHost   string
	proxy         string
	keepAliveIdle time.Duration
	writeTimeout  time.Duration
	handlerSlots  chan struct{}
	maxHandlers   int
}

type connectionContextKey struct{}

const (
	defaultWriteTimeout       = 5 * time.Second
	defaultMaxPendingHandlers = 100
)

func NewStreamClient(options ...ClientOption) *StreamClient {
	lifecycleCtx, cancel := context.WithCancel(context.Background())
	cli := &StreamClient{
		keepAliveIdle: 120 * time.Second,
		writeTimeout:  defaultWriteTimeout,
		maxHandlers:   defaultMaxPendingHandlers,
		lifecycleCtx:  lifecycleCtx,
		cancel:        cancel,
	}

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
	cli.handlerSlots = make(chan struct{}, cli.maxHandlers)

	return cli
}

func (cli *StreamClient) Start(ctx context.Context) error {
	return cli.start(ctx, nil)
}

func (cli *StreamClient) start(ctx context.Context, expectedLifecycle context.Context) error {
	cli.startMutex.Lock()
	defer cli.startMutex.Unlock()

	cli.mutex.Lock()
	if cli.conn != nil {
		cli.mutex.Unlock()
		return nil
	}
	if expectedLifecycle == nil {
		if cli.lifecycleCtx == nil || cli.lifecycleCtx.Err() != nil {
			cli.lifecycleCtx, cli.cancel = context.WithCancel(context.Background())
		}
	} else if cli.lifecycleCtx != expectedLifecycle || expectedLifecycle.Err() != nil {
		cli.mutex.Unlock()
		return context.Canceled
	}
	lifecycleCtx := cli.lifecycleCtx
	cli.mutex.Unlock()

	connectCtx, cancelConnect := contextWithLifecycle(ctx, lifecycleCtx)
	defer cancelConnect()

	endpoint, err := cli.GetConnectionEndpoint(connectCtx)
	if err != nil {
		return err
	}

	wssUrl := fmt.Sprintf("%s?ticket=%s", endpoint.Endpoint, endpoint.Ticket)

	header := make(http.Header)

	var dialer *websocket.Dialer

	if len(cli.proxy) == 0 {
		dialer = websocket.DefaultDialer
	} else {
		proxyURL, err := url.Parse(cli.proxy)
		if err != nil {
			return err
		}
		dialer = &websocket.Dialer{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	conn, resp, err := dialer.DialContext(connectCtx, wssUrl, header)
	if err != nil {
		closeWebSocketDialResources(conn, resp)
		return err
	}

	// 建连失败
	if resp != nil && resp.StatusCode >= http.StatusBadRequest {
		_ = conn.Close()
		return utils.ErrorFromHttpResponseBody(resp)
	}

	cli.mutex.Lock()
	if cli.lifecycleCtx != lifecycleCtx || lifecycleCtx.Err() != nil {
		cli.mutex.Unlock()
		_ = conn.Close()
		return context.Canceled
	}
	if cli.conn != nil {
		cli.mutex.Unlock()
		_ = conn.Close()
		return nil
	}
	cli.conn = conn
	cli.sessionId = endpoint.Ticket
	keepAliveIdle := cli.keepAliveIdle
	sessionID := cli.sessionId
	cli.mutex.Unlock()

	logger.GetLogger().Infof("connect success, sessionId=[%s]", sessionID)

	go cli.processConnection(conn, keepAliveIdle)

	return nil
}

func contextWithLifecycle(ctx, lifecycleCtx context.Context) (context.Context, context.CancelFunc) {
	connectCtx, cancel := context.WithCancel(ctx)
	go func() {
		select {
		case <-lifecycleCtx.Done():
			cancel()
		case <-connectCtx.Done():
		}
	}()
	return connectCtx, cancel
}

func (cli *StreamClient) processLoop() {
	cli.mutex.Lock()
	conn := cli.conn
	keepAliveIdle := cli.keepAliveIdle
	cli.mutex.Unlock()

	cli.processConnection(conn, keepAliveIdle)
}

func (cli *StreamClient) processConnection(conn *websocket.Conn, keepAliveIdle time.Duration) {
	defer func() {
		if err := recover(); err != nil {
			logger.GetLogger().Errorf("connection process panic due to unknown reason, error=[%s]", err)
		}
		lifecycleCtx, shouldReconnect := cli.detachConnection(conn)
		cli.closeConnection(conn)
		if shouldReconnect {
			go cli.reconnect(lifecycleCtx)
		}
	}()

	if conn == nil {
		logger.GetLogger().Errorf("connection process connect nil, maybe disconnected.")
		return
	}

	// Use context instead of closeChan to avoid send-on-closed-channel panic.
	// see: https://github.com/open-dingtalk/dingtalk-stream-sdk-go/issues/27
	// see: https://github.com/open-dingtalk/dingtalk-stream-sdk-go/issues/28
	// see: https://github.com/open-dingtalk/dingtalk-stream-sdk-go/issues/32
	loopCtx, cancelLoop := context.WithCancel(context.Background())
	defer cancelLoop()

	// Buffered + non-blocking send: late/extra pongs must never block ReadMessage.
	pongReceived := make(chan struct{}, 1)
	conn.SetPongHandler(func(string) error {
		select {
		case pongReceived <- struct{}{}:
		default:
		}
		return nil
	})

	readChan := make(chan []byte)
	// 读消息 goroutine，退出时关闭 readChan 通知主循环
	go func() {
		defer func() {
			if err := recover(); err != nil {
				logger.GetLogger().Errorf("connection read loop panic, error=[%s]", err)
			}
			close(readChan)
			cancelLoop()
		}()
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				logger.GetLogger().Errorf("connection process read message error: messageType=[%d] message=[%s] error=[%s]", messageType, string(message), err)
				return
			}
			if messageType == websocket.TextMessage {
				select {
				case readChan <- message:
				case <-loopCtx.Done():
					return
				}
			}
		}
	}()

	const pingWait = 5 * time.Second

	for {
		var idleTimer <-chan time.Time
		var timer *time.Timer
		if keepAliveIdle > 0 {
			timer = time.NewTimer(keepAliveIdle)
			idleTimer = timer.C
		}

		select {
		case msg, ok := <-readChan:
			stopTimer(timer)
			if !ok {
				logger.GetLogger().Errorf("connection process is closed")
				return
			}
			cli.dispatchDataFrameForConnectionContext(loopCtx, conn, msg)

		case <-idleTimer:
			// Drain a stale pong signal from a previous round.
			select {
			case <-pongReceived:
			default:
			}

			if err := cli.writeControlToConnection(conn, websocket.PingMessage, nil); err != nil {
				logger.GetLogger().Errorf("connection write ping message error: error=[%s]", err)
				return
			}

			// Wait for pong synchronously so we never race multiple waiters on one channel.
			select {
			case <-pongReceived:
				// alive
			case msg, ok := <-readChan:
				// Any inbound traffic means the connection is still up.
				if !ok {
					logger.GetLogger().Errorf("connection process is closed")
					return
				}
				cli.dispatchDataFrameForConnectionContext(loopCtx, conn, msg)
			case <-time.After(pingWait):
				logger.GetLogger().Errorf("ping time out, connection is closing")
				_ = conn.Close()
				cancelLoop()
				return
			case <-loopCtx.Done():
				return
			}

		case <-loopCtx.Done():
			stopTimer(timer)
			return
		}
	}
}

func stopTimer(timer *time.Timer) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func (cli *StreamClient) writeMessage(messageType int, data []byte) error {
	cli.mutex.Lock()
	conn := cli.conn
	cli.mutex.Unlock()

	return cli.writeMessageToConnection(conn, messageType, data)
}

func (cli *StreamClient) writeMessageToConnection(conn *websocket.Conn, messageType int, data []byte) error {
	if conn == nil {
		return errors.New("disconnected")
	}

	cli.writeMutex.Lock()
	defer cli.writeMutex.Unlock()
	if err := conn.SetWriteDeadline(time.Now().Add(cli.getWriteTimeout())); err != nil {
		return err
	}
	return conn.WriteMessage(messageType, data)
}

func (cli *StreamClient) writeControlToConnection(conn *websocket.Conn, messageType int, data []byte) error {
	if conn == nil {
		return errors.New("disconnected")
	}

	// Gorilla permits WriteControl concurrently with data writes. Keeping
	// heartbeat control frames out of writeMutex ensures a blocked application
	// write cannot prevent keepalive from timing out and closing the connection.
	return conn.WriteControl(messageType, data, time.Now().Add(cli.getWriteTimeout()))
}

func (cli *StreamClient) writeJSON(v interface{}) error {
	cli.mutex.Lock()
	conn := cli.conn
	cli.mutex.Unlock()

	return cli.writeJSONToConnection(conn, v)
}

func (cli *StreamClient) writeJSONToConnection(conn *websocket.Conn, v interface{}) error {
	if conn == nil {
		return errors.New("disconnected")
	}

	cli.writeMutex.Lock()
	defer cli.writeMutex.Unlock()
	if err := conn.SetWriteDeadline(time.Now().Add(cli.getWriteTimeout())); err != nil {
		return err
	}
	return conn.WriteJSON(v)
}

func (cli *StreamClient) getWriteTimeout() time.Duration {
	if cli.writeTimeout <= 0 {
		return defaultWriteTimeout
	}
	return cli.writeTimeout
}

func (cli *StreamClient) processDataFrame(rawData []byte) {
	cli.mutex.Lock()
	conn := cli.conn
	cli.mutex.Unlock()

	cli.processDataFrameForConnection(conn, rawData)
}

func (cli *StreamClient) dispatchDataFrameForConnection(conn *websocket.Conn, rawData []byte) {
	cli.dispatchDataFrameForConnectionContext(context.Background(), conn, rawData)
}

func (cli *StreamClient) dispatchDataFrameForConnectionContext(
	ctx context.Context,
	conn *websocket.Conn,
	rawData []byte,
) {
	select {
	case cli.handlerSlots <- struct{}{}:
		go func() {
			defer func() {
				<-cli.handlerSlots
			}()
			cli.processDataFrameForConnectionContext(ctx, conn, rawData)
		}()
	default:
		dataFrame, err := payload.DecodeDataFrame(rawData)
		if err == nil && dataFrame != nil && dataFrame.Type == utils.SubscriptionTypeKSystem {
			// System frames control connection lifecycle and must not be starved
			// by slow application handlers. Process them synchronously to keep
			// control-frame work bounded even if the server sends a burst.
			cli.processDataFrameForConnectionContext(ctx, conn, rawData)
			return
		}
		logger.GetLogger().Warningf(
			"connection handler capacity reached, message is left unacknowledged for retry",
		)
	}
}

func (cli *StreamClient) processDataFrameForConnection(conn *websocket.Conn, rawData []byte) {
	cli.processDataFrameForConnectionContext(context.Background(), conn, rawData)
}

func (cli *StreamClient) processDataFrameForConnectionContext(
	ctx context.Context,
	conn *websocket.Conn,
	rawData []byte,
) {
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

	var dataAck *payload.DataFrameResponse
	frameHandler, err := cli.GetHandler(dataFrame.Type, dataFrame.GetTopic())
	if err != nil || frameHandler == nil {
		// 没有注册handler，返回404
		dataAck = payload.NewDataFrameResponse(payload.DataFrameResponseStatusCodeKHandlerNotFound)
	} else {
		if ctx == nil {
			ctx = context.Background()
		}
		frameContext := context.WithValue(ctx, connectionContextKey{}, conn)
		dataAck, err = frameHandler(frameContext, dataFrame)

		if err != nil && dataAck == nil {
			dataAck = payload.NewErrorDataFrameResponse(err)
		}
	}

	if dataAck == nil {
		dataAck = payload.NewSuccessDataFrameResponse()
	}

	if dataAck.GetHeader(payload.DataFrameHeaderKMessageId) == "" {
		dataAck.SetHeader(payload.DataFrameHeaderKMessageId, dataFrame.GetMessageId())
	}

	if dataAck.GetHeader(payload.DataFrameHeaderKContentType) == "" {
		dataAck.SetHeader(payload.DataFrameHeaderKContentType, payload.DataFrameContentTypeKJson)
	}

	errSend := cli.sendDataFrameResponse(conn, dataAck)
	sentBytes, _ := json.Marshal(dataAck)
	logger.GetLogger().Debugf("[wire] [websocket] local => remote:\n%s", string(sentBytes))

	if errSend != nil {
		logger.GetLogger().Errorf("connection processDataFrame send response error: error=[%s]", errSend)
	}
	if dataFrame.Type == utils.SubscriptionTypeKSystem && dataFrame.GetTopic() == "disconnect" {
		// Acknowledge the server command on the source connection before
		// closing it. Closing inside OnDisconnect makes the ACK write fail and
		// causes the server to retry an already-processed disconnect command.
		cli.closeOwnedConnection(conn)
	}
}

func closeWebSocketDialResources(conn *websocket.Conn, resp *http.Response) {
	if conn != nil {
		_ = conn.Close()
	}
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}

func (cli *StreamClient) Close() {
	cli.mutex.Lock()
	conn := cli.conn
	cli.conn = nil
	cli.sessionId = ""
	if cli.cancel != nil {
		cli.cancel()
	}
	cli.mutex.Unlock()

	if conn != nil {
		if err := conn.Close(); err != nil {
			logger.GetLogger().Errorf("StreamClient close. error=[%s]", err)
		}
	}
}

func (cli *StreamClient) detachConnection(conn *websocket.Conn) (context.Context, bool) {
	if conn == nil {
		return nil, false
	}

	cli.mutex.Lock()
	defer cli.mutex.Unlock()
	if cli.conn != conn {
		return nil, false
	}

	cli.conn = nil
	cli.sessionId = ""
	return cli.lifecycleCtx, cli.AutoReconnect
}

func (cli *StreamClient) closeConnection(conn *websocket.Conn) {
	if conn == nil {
		return
	}
	if err := conn.Close(); err != nil {
		logger.GetLogger().Errorf("StreamClient close. error=[%s]", err)
	}
}

func (cli *StreamClient) reconnect(lifecycleCtx context.Context) {
	defer func() {
		if err := recover(); err != nil {
			logger.GetLogger().Errorf("reconect panic due to unknown reason. error=[%s]", err)
		}
	}()

	// Backoff before retrying to avoid credential reconnect storms / server-side throttling.
	backoff := 3 * time.Second
	const maxBackoff = 60 * time.Second
	random := rand.New(rand.NewSource(time.Now().UnixNano()))

	for attempt := 1; ; attempt++ {
		jitter := time.Duration(random.Int63n(int64(backoff/5) + 1))
		timer := time.NewTimer(backoff + jitter)
		select {
		case <-timer.C:
		case <-lifecycleCtx.Done():
			stopTimer(timer)
			return
		}

		err := cli.start(lifecycleCtx, lifecycleCtx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			logger.GetLogger().Errorf("StreamClient reconnect error, attempt=[%d] error=[%s]", attempt, err)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		logger.GetLogger().Infof("StreamClient reconnect success")
		return
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
		Extras:        cli.extras,
	}
	if localIp, err := utils.GetFirstLanIP(); err == nil {
		requestModel.LocalIP = localIp
	}

	for ttype, subs := range cli.subscriptions {
		for ttopic := range subs {
			requestModel.Subscriptions = append(requestModel.Subscriptions, &payload.SubscriptionModel{
				Type:  ttype,
				Topic: ttopic,
			})
		}
	}

	requestJsonBody, _ := json.Marshal(requestModel)

	var targetHost string
	if len(cli.openApiHost) == 0 {
		targetHost = utils.DefaultOpenApiHost
	} else {
		targetHost = cli.openApiHost
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetHost+utils.GetConnectionEndpointAPIUrl, bytes.NewReader(requestJsonBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	var transport http.RoundTripper
	if len(cli.proxy) == 0 {
		transport = http.DefaultTransport
	} else {
		proxyURL, err := url.Parse(cli.proxy)

		if err != nil {
			return nil, err
		}
		transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   5 * time.Second, //设置超时，包含connection时间、任意重定向时间、读取response body时间
	}

	logger.GetLogger().Debugf("[wire] [http] local => remote:\n%s %s %s\nHost: %s\n%s\n\n%s",
		req.Method, req.URL.RequestURI(), req.Proto, req.Host,
		utils.DumpHeaders(req.Header), requestJsonBody)
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
	logger.GetLogger().Debugf("[wire] [http] remote => localhost:\n%s %s\n%s\n\n%s",
		resp.Proto, resp.Status,
		utils.DumpHeaders(resp.Header), responseJsonBody)

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

	if ctx != nil {
		if _, ok := ctx.Value(connectionContextKey{}).(*websocket.Conn); ok {
			return nil, nil
		}
	}
	cli.Close()
	return nil, nil
}

func (cli *StreamClient) closeOwnedConnection(conn *websocket.Conn) {
	// Close only the connection that carried the disconnect frame. Its process
	// loop decides whether it still owns the client state and should reconnect.
	cli.closeConnection(conn)
}

func (cli *StreamClient) OnPing(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
	dfPong := payload.NewDataFrameAckPong(df.GetMessageId())
	dfPong.Data = df.Data

	return dfPong, nil
}

// 返回正常数据包
func (cli *StreamClient) SendDataFrameResponse(ctx context.Context, resp *payload.DataFrameResponse) error {
	if ctx != nil {
		if conn, ok := ctx.Value(connectionContextKey{}).(*websocket.Conn); ok {
			return cli.sendDataFrameResponse(conn, resp)
		}
	}

	cli.mutex.Lock()
	conn := cli.conn
	cli.mutex.Unlock()

	return cli.sendDataFrameResponse(conn, resp)
}

func (cli *StreamClient) sendDataFrameResponse(conn *websocket.Conn, resp *payload.DataFrameResponse) error {
	if resp == nil {
		return errors.New("SendDataFrameResponseError_ResponseNil")
	}

	if err := cli.writeJSONToConnection(conn, resp); err != nil {
		logger.GetLogger().Errorf("SendDataFrameResponse error, conn nil or write failed, error=[%s]", err)
		return err
	}
	return nil
}

// 通用注册函数
func (cli *StreamClient) RegisterRouter(stype, stopic string, frameHandler handler.IFrameHandler) {
	if cli.subscriptions == nil {
		cli.subscriptions = make(map[string]map[string]handler.IFrameHandler)
	}

	if _, ok := cli.subscriptions[stype]; !ok {
		cli.subscriptions[stype] = make(map[string]handler.IFrameHandler)
	}

	cli.subscriptions[stype][stopic] = frameHandler
}

// callback类型注册函数
func (cli *StreamClient) RegisterCallbackRouter(topic string, frameHandler handler.IFrameHandler) {
	cli.RegisterRouter(utils.SubscriptionTypeKCallback, topic, frameHandler)
}

// 聊天机器人的注册函数
func (cli *StreamClient) RegisterChatBotCallbackRouter(messageHandler chatbot.IChatBotMessageHandler) {
	cli.RegisterRouter(utils.SubscriptionTypeKCallback, payload.BotMessageCallbackTopic, chatbot.NewDefaultChatBotFrameHandler(messageHandler).OnEventReceived)
}

// AI插件的注册函数
func (cli *StreamClient) RegisterPluginCallbackRouter(messageHandler plugin.IPluginMessageHandler) {
	cli.RegisterRouter(utils.SubscriptionTypeKCallback, payload.PluginMessageCallbackTopic, plugin.NewDefaultPluginFrameHandler(messageHandler).OnEventReceived)
}

// 互动卡片的注册函数
func (cli *StreamClient) RegisterCardCallbackRouter(messageHandler card.ICardCallbackHandler) {
	cli.RegisterRouter(utils.SubscriptionTypeKCallback, payload.CardInstanceCallbackTopic, card.NewDefaultPluginFrameHandler(messageHandler).OnEventReceived)
}

// 事件类型的注册函数
func (cli *StreamClient) RegisterEventRouter(topic string, frameHandler handler.IFrameHandler) {
	cli.RegisterRouter(utils.SubscriptionTypeKEvent, topic, frameHandler)
}

// 所有事件的注册函数
func (cli *StreamClient) RegisterAllEventRouter(frameHandler handler.IFrameHandler) {
	cli.RegisterRouter(utils.SubscriptionTypeKEvent, "*", frameHandler)
}
