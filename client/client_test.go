package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/utils"
)

/**
 * @Author linya.jj
 * @Date 2023/3/22 14:23
 */

func TestNewDingtalkOpenStreamClient(t *testing.T) {
	c := NewStreamClient()
	assert.Equal(t, 120*time.Second, c.keepAliveIdle)
	assert.True(t, c.AutoReconnect)

	h, err := c.GetHandler(utils.SubscriptionTypeKSystem, "ping")
	assert.NoError(t, err)
	assert.NotNil(t, h)

	h, err = c.GetHandler(utils.SubscriptionTypeKSystem, "disconnect")
	assert.NoError(t, err)
	assert.NotNil(t, h)
}

func TestDingtalkOpenStreamClient_Start(t *testing.T) {
	wsEndpoint, closeWS := startEchoWSServer(t)
	defer closeWS()

	openAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, utils.GetConnectionEndpointAPIUrl, r.URL.Path)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"endpoint": wsEndpoint,
			"ticket":   "ticket-start-1",
		})
	}))
	defer openAPI.Close()

	cli := NewStreamClient(
		WithAppCredential(NewAppCredentialConfig("cid", "csecret")),
		WithOpenApiHost(openAPI.URL),
		WithAutoReconnect(false),
		WithKeepAlive(0),
	)
	require.NoError(t, cli.Start(context.Background()))
	require.NotNil(t, cli.conn)
	require.Equal(t, "ticket-start-1", cli.sessionId)

	// Second Start is a no-op when already connected.
	require.NoError(t, cli.Start(context.Background()))
	cli.Close()
	require.Nil(t, cli.conn)
}

func TestDingtalkOpenStreamClient_processDataFrame(t *testing.T) {
	var got atomic.Value
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		got.Store(string(data))
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL), nil)
	require.NoError(t, err)

	cli := NewStreamClient(WithAutoReconnect(false))
	cli.conn = conn

	raw := []byte(`{"headers":{"messageId":"m-1","topic":"ping"},"type":"SYSTEM","data":"{\"hello\":\"world\"}"}`)
	cli.processDataFrame(raw)

	require.Eventually(t, func() bool {
		v, _ := got.Load().(string)
		return strings.Contains(v, "m-1")
	}, time.Second, 10*time.Millisecond)

	cli.Close()
}

func TestApplicationHandlerConcurrencyIsBounded(t *testing.T) {
	cli := NewStreamClient(
		WithAutoReconnect(false),
		WithMaxPendingHandlers(5),
	)
	var handlerCalls int32
	releaseHandlers := make(chan struct{})
	cli.RegisterRouter(
		utils.SubscriptionTypeKCallback,
		"/slow",
		func(context.Context, *payload.DataFrame) (*payload.DataFrameResponse, error) {
			atomic.AddInt32(&handlerCalls, 1)
			<-releaseHandlers
			return payload.NewSuccessDataFrameResponse(), nil
		},
	)

	for i := 0; i < 20; i++ {
		raw := []byte(fmt.Sprintf(
			`{"headers":{"messageId":"m-%d","topic":"/slow"},"type":"CALLBACK","data":"{}"}`,
			i,
		))
		cli.dispatchDataFrameForConnection(nil, raw)
	}

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&handlerCalls) == 5
	}, time.Second, 10*time.Millisecond)
	assert.Equal(t, int32(5), atomic.LoadInt32(&handlerCalls))

	close(releaseHandlers)
	require.Eventually(t, func() bool {
		return len(cli.handlerSlots) == 0
	}, time.Second, 10*time.Millisecond)
}

func TestApplicationHandlerContextIsCanceledWithConnection(t *testing.T) {
	cli := NewStreamClient(
		WithAutoReconnect(false),
		WithMaxPendingHandlers(1),
	)
	handlerStarted := make(chan struct{})
	handlerCanceled := make(chan struct{})
	cli.RegisterRouter(
		utils.SubscriptionTypeKCallback,
		"/cancel-aware",
		func(ctx context.Context, _ *payload.DataFrame) (*payload.DataFrameResponse, error) {
			close(handlerStarted)
			<-ctx.Done()
			close(handlerCanceled)
			return nil, ctx.Err()
		},
	)

	connectionCtx, cancelConnection := context.WithCancel(context.Background())
	raw := []byte(
		`{"headers":{"messageId":"cancel-1","topic":"/cancel-aware"},"type":"CALLBACK","data":"{}"}`,
	)
	cli.dispatchDataFrameForConnectionContext(connectionCtx, nil, raw)

	select {
	case <-handlerStarted:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}
	cancelConnection()
	select {
	case <-handlerCanceled:
	case <-time.After(time.Second):
		t.Fatal("handler did not observe connection cancellation")
	}
	require.Eventually(t, func() bool {
		return len(cli.handlerSlots) == 0
	}, time.Second, 10*time.Millisecond)
}

func TestCloseWebSocketDialResourcesClosesHandshakeResponse(t *testing.T) {
	body := &trackingReadCloser{Reader: strings.NewReader("handshake failed")}
	closeWebSocketDialResources(nil, &http.Response{Body: body})
	assert.True(t, body.closed)
}

func TestDingtalkOpenStreamClient_Close(t *testing.T) {
	cli := NewStreamClient(WithAutoReconnect(false))
	cli.Close() // nil conn is safe

	_, closeFn, conn := dialTestWS(t)
	defer closeFn()

	cli.conn = conn
	cli.sessionId = "s1"
	cli.Close()
	assert.Nil(t, cli.conn)
	assert.Equal(t, "", cli.sessionId)
	cli.Close() // idempotent
}

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (body *trackingReadCloser) Close() error {
	body.closed = true
	return nil
}

func TestDingtalkOpenStreamClient_reconnect(t *testing.T) {
	var opens int64
	wsEndpoint, closeWS := startEchoWSServer(t)
	defer closeWS()

	openAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt64(&opens, 1)
		if n == 1 {
			http.Error(w, `{"code":"Throttling","message":"busy"}`, http.StatusTooManyRequests)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"endpoint": wsEndpoint,
			"ticket":   fmt.Sprintf("ticket-%d", n),
		})
	}))
	defer openAPI.Close()

	cli := NewStreamClient(
		WithAppCredential(NewAppCredentialConfig("cid", "csecret")),
		WithOpenApiHost(openAPI.URL),
		WithAutoReconnect(false),
		WithKeepAlive(0),
	)

	done := make(chan struct{})
	go func() {
		cli.reconnect(cli.lifecycleCtx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(12 * time.Second):
		t.Fatal("reconnect did not succeed with backoff")
	}

	assert.GreaterOrEqual(t, atomic.LoadInt64(&opens), int64(2))
	assert.NotNil(t, cli.conn)
	cli.Close()
}

func TestDingtalkOpenStreamClient_GetHandler(t *testing.T) {
	cli := NewStreamClient()
	_, err := cli.GetHandler("UNKNOWN", "x")
	assert.Error(t, err)

	cli.RegisterCallbackRouter("/custom", func(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
		return payload.NewSuccessDataFrameResponse(), nil
	})
	h, err := cli.GetHandler(utils.SubscriptionTypeKCallback, "/custom")
	assert.NoError(t, err)
	assert.NotNil(t, h)
}

func TestDingtalkOpenStreamClient_CheckConfigValid(t *testing.T) {
	cli := NewStreamClient()
	assert.Error(t, cli.CheckConfigValid())

	cli = NewStreamClient(WithAppCredential(NewAppCredentialConfig("cid", "sec")))
	assert.NoError(t, cli.CheckConfigValid())

	cli.RegisterRouter("BADTYPE", "t", func(ctx context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
		return nil, nil
	})
	assert.Error(t, cli.CheckConfigValid())

	cli = NewStreamClient(WithAppCredential(NewAppCredentialConfig("cid", "sec")))
	cli.RegisterRouter(utils.SubscriptionTypeKCallback, "nil-handler", nil)
	assert.Error(t, cli.CheckConfigValid())
}

func TestDingtalkOpenStreamClient_GetConnectionEndpoint(t *testing.T) {
	openAPI := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req payload.ConnectionEndpointRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		assert.Equal(t, "cid", req.ClientId)
		assert.Equal(t, "sec", req.ClientSecret)
		assert.NotEmpty(t, req.Subscriptions)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"endpoint": "wss://example.dingtalk.com/connect",
			"ticket":   "ticket-abc",
		})
	}))
	defer openAPI.Close()

	cli := NewStreamClient(
		WithAppCredential(NewAppCredentialConfig("cid", "sec")),
		WithOpenApiHost(openAPI.URL),
	)
	ep, err := cli.GetConnectionEndpoint(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "wss://example.dingtalk.com/connect", ep.Endpoint)
	assert.Equal(t, "ticket-abc", ep.Ticket)
}

func TestDingtalkOpenStreamClient_OnDisconnect(t *testing.T) {
	_, closeFn, conn := dialTestWS(t)
	defer closeFn()

	cli := NewStreamClient(WithAutoReconnect(false))
	cli.conn = conn
	cli.sessionId = "sid"

	_, err := cli.OnDisconnect(context.Background(), &payload.DataFrame{
		Type: utils.SubscriptionTypeKSystem,
		Headers: payload.DataFrameHeader{
			payload.DataFrameHeaderKTopic:     "disconnect",
			payload.DataFrameHeaderKMessageId: "d1",
		},
	})
	assert.NoError(t, err)
	assert.Nil(t, cli.conn)
	assert.Equal(t, "", cli.sessionId)
}

func TestDingtalkOpenStreamClient_OnPing(t *testing.T) {
	cli := NewStreamClient()
	df := &payload.DataFrame{
		Type: utils.SubscriptionTypeKSystem,
		Headers: payload.DataFrameHeader{
			payload.DataFrameHeaderKTopic:     "ping",
			payload.DataFrameHeaderKMessageId: "ping-1",
		},
		Data: `{"ts":1}`,
	}
	resp, err := cli.OnPing(context.Background(), df)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "ping-1", resp.GetHeader(payload.DataFrameHeaderKMessageId))
	assert.Equal(t, `{"ts":1}`, resp.Data)
}

func TestDingtalkOpenStreamClient_SendDataFrameResponse(t *testing.T) {
	cli := NewStreamClient(WithAutoReconnect(false))
	assert.Error(t, cli.SendDataFrameResponse(context.Background(), nil))

	var got atomic.Value
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		got.Store(string(data))
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL), nil)
	require.NoError(t, err)
	cli.conn = conn

	resp := payload.NewSuccessDataFrameResponse()
	resp.SetHeader(payload.DataFrameHeaderKMessageId, "ack-1")
	require.NoError(t, cli.SendDataFrameResponse(context.Background(), resp))
	require.Eventually(t, func() bool {
		v, _ := got.Load().(string)
		return strings.Contains(v, "ack-1")
	}, time.Second, 10*time.Millisecond)
	cli.Close()
}

func TestDingtalkOpenStreamClient_SendErrorResponse(t *testing.T) {
	cli := NewStreamClient(WithAutoReconnect(false))
	assert.EqualError(t, cli.SendDataFrameResponse(context.Background(), nil), "SendDataFrameResponseError_ResponseNil")
}

func TestStopTimer(t *testing.T) {
	stopTimer(nil) // no panic

	tm := time.NewTimer(time.Hour)
	stopTimer(tm)

	tm2 := time.NewTimer(time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	stopTimer(tm2) // already fired
}

func TestProcessLoopMessageDuringPingWaitKeepsAlive(t *testing.T) {
	// If app traffic arrives while waiting for WS pong, connection should stay up
	// even when server never replies to ping (old false-timeout risk path).
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetPingHandler(func(string) error { return nil }) // no pong

		go func() {
			time.Sleep(40 * time.Millisecond)
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"headers":{"messageId":"keep","topic":"ping"},"type":"SYSTEM","data":"{}"}`))
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL), nil)
	require.NoError(t, err)

	cli := NewStreamClient(WithAutoReconnect(false))
	cli.conn = conn
	cli.keepAliveIdle = 20 * time.Millisecond

	done := make(chan struct{})
	go func() {
		cli.processLoop()
		close(done)
	}()

	// Should still be alive after one idle+ping-wait cycle rescued by inbound message.
	time.Sleep(200 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("processLoop exited too early; inbound message should keep connection alive")
	default:
	}

	cli.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("processLoop did not exit after Close")
	}
}

func TestProcessLoopPingPongKeepsConnection(t *testing.T) {
	var gotPing uint32
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		conn.SetPingHandler(func(appData string) error {
			atomic.StoreUint32(&gotPing, 1)
			return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
		})

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL), nil)
	require.NoError(t, err)

	cli := NewStreamClient(WithAutoReconnect(false))
	cli.conn = conn
	cli.keepAliveIdle = 50 * time.Millisecond

	done := make(chan struct{})
	go func() {
		cli.processLoop()
		close(done)
	}()

	require.Eventually(t, func() bool {
		return atomic.LoadUint32(&gotPing) == 1
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, cli.writeMessage(websocket.TextMessage, []byte(`{"type":"SYSTEM"}`)))

	cli.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("processLoop did not exit after Close")
	}
}

func TestProcessLoopPingTimeoutClosesConnection(t *testing.T) {
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetPingHandler(func(string) error { return nil })
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL), nil)
	require.NoError(t, err)

	cli := NewStreamClient(WithAutoReconnect(false))
	cli.conn = conn
	cli.keepAliveIdle = 20 * time.Millisecond

	done := make(chan struct{})
	go func() {
		cli.processLoop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(6 * time.Second):
		t.Fatal("processLoop did not exit on ping timeout")
	}
}

func TestProcessLoopLatePongDoesNotBlockReadPath(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_ = conn.WriteControl(websocket.PongMessage, nil, time.Now().Add(time.Second))
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"headers":{"messageId":"1","topic":"ping"},"type":"SYSTEM","data":"{}"}`))
		time.Sleep(200 * time.Millisecond)
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL), nil)
	require.NoError(t, err)

	cli := NewStreamClient(WithAutoReconnect(false), WithKeepAlive(0))
	cli.conn = conn

	done := make(chan struct{})
	go func() {
		cli.processLoop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("processLoop blocked after unsolicited pong")
	}
}

func TestProcessLoopConcurrentMessagesAndPing(t *testing.T) {
	var (
		gotPing   int64
		gotClient int64
	)
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		conn.SetPingHandler(func(appData string) error {
			atomic.AddInt64(&gotPing, 1)
			return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
		})

		go func() {
			for i := 0; i < 50; i++ {
				msg := fmt.Sprintf(`{"headers":{"messageId":"%d","topic":"ping"},"type":"SYSTEM","data":"{}"}`, i)
				if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
		}()

		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
			atomic.AddInt64(&gotClient, 1)
		}
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL), nil)
	require.NoError(t, err)

	cli := NewStreamClient(WithAutoReconnect(false))
	cli.conn = conn
	cli.keepAliveIdle = 30 * time.Millisecond

	done := make(chan struct{})
	go func() {
		cli.processLoop()
		close(done)
	}()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = cli.writeJSON(map[string]interface{}{
				"code": 200,
				"headers": map[string]string{
					"messageId": fmt.Sprintf("ack-%d", i),
				},
			})
		}(i)
	}
	wg.Wait()

	require.Eventually(t, func() bool { return atomic.LoadInt64(&gotPing) >= 1 }, 2*time.Second, 20*time.Millisecond)
	require.Eventually(t, func() bool { return atomic.LoadInt64(&gotClient) >= 1 }, 2*time.Second, 20*time.Millisecond)

	cli.Close()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("processLoop did not exit after concurrent traffic")
	}
}

func TestProcessLoopUnsolicitedPongFlood(t *testing.T) {
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		for i := 0; i < 100; i++ {
			if err := conn.WriteControl(websocket.PongMessage, nil, time.Now().Add(time.Second)); err != nil {
				return
			}
		}
		_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"headers":{"messageId":"flood","topic":"ping"},"type":"SYSTEM","data":"{}"}`))
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL), nil)
	require.NoError(t, err)

	cli := NewStreamClient(WithAutoReconnect(false), WithKeepAlive(0))
	cli.conn = conn

	done := make(chan struct{})
	go func() {
		cli.processLoop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("processLoop blocked under unsolicited pong flood")
	}
}

func TestProcessLoopAppLevelPingWithKeepAliveDisabled(t *testing.T) {
	var gotPong uint32
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		pingFrame := `{"headers":{"messageId":"app-ping","topic":"ping"},"type":"SYSTEM","data":"{}"}`
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(pingFrame)))

		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if strings.Contains(string(data), "app-ping") {
			atomic.StoreUint32(&gotPong, 1)
		}
		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL), nil)
	require.NoError(t, err)

	cli := NewStreamClient(WithAutoReconnect(false), WithKeepAlive(0))
	cli.conn = conn

	done := make(chan struct{})
	go func() {
		cli.processLoop()
		close(done)
	}()

	require.Eventually(t, func() bool {
		return atomic.LoadUint32(&gotPong) == 1
	}, time.Second, 10*time.Millisecond)

	cli.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("processLoop did not exit")
	}
}

func TestProcessLoopMultiplePingRounds(t *testing.T) {
	var pingCount int64
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		conn.SetPingHandler(func(appData string) error {
			atomic.AddInt64(&pingCount, 1)
			return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL), nil)
	require.NoError(t, err)

	cli := NewStreamClient(WithAutoReconnect(false))
	cli.conn = conn
	cli.keepAliveIdle = 40 * time.Millisecond

	done := make(chan struct{})
	go func() {
		cli.processLoop()
		close(done)
	}()

	require.Eventually(t, func() bool { return atomic.LoadInt64(&pingCount) >= 3 }, 2*time.Second, 20*time.Millisecond)

	cli.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("processLoop did not exit after multiple ping rounds")
	}
}

func TestWriteHelpersRejectAfterClose(t *testing.T) {
	cli := NewStreamClient(WithAutoReconnect(false))
	require.EqualError(t, cli.writeMessage(websocket.TextMessage, []byte("x")), "disconnected")
	require.EqualError(t, cli.writeJSON(map[string]string{"a": "b"}), "disconnected")
	require.EqualError(t, cli.SendDataFrameResponse(context.Background(), payload.NewSuccessDataFrameResponse()), "disconnected")
}

func TestConcurrentWritesSerialized(t *testing.T) {
	var reads int64
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
			atomic.AddInt64(&reads, 1)
		}
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL), nil)
	require.NoError(t, err)

	cli := NewStreamClient(WithAutoReconnect(false))
	cli.conn = conn

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = cli.writeJSON(map[string]interface{}{"i": i})
			_ = cli.writeMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"n":%d}`, i)))
		}(i)
	}
	wg.Wait()

	require.Eventually(t, func() bool { return atomic.LoadInt64(&reads) >= 50 }, 2*time.Second, 10*time.Millisecond)
	cli.Close()
}

func TestCloseDoesNotWaitForBlockedWrite(t *testing.T) {
	upgrader := websocket.Upgrader{}
	releaseServer := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// Do not read: a sufficiently large client write will block until Close
		// closes the socket.
		<-releaseServer
	}))
	defer server.Close()
	defer close(releaseServer)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL), nil)
	require.NoError(t, err)

	cli := NewStreamClient(WithAutoReconnect(false))
	cli.conn = conn

	writeStarted := make(chan struct{})
	writeDone := make(chan struct{})
	go func() {
		close(writeStarted)
		_ = cli.writeMessage(websocket.BinaryMessage, make([]byte, 64<<20))
		close(writeDone)
	}()
	<-writeStarted
	time.Sleep(100 * time.Millisecond)

	closeDone := make(chan struct{})
	go func() {
		cli.Close()
		close(closeDone)
	}()

	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("Close blocked behind an in-flight websocket write")
	}

	select {
	case <-writeDone:
	case <-time.After(time.Second):
		t.Fatal("blocked websocket write did not exit after Close")
	}
}

func TestBlockedDataWriteDoesNotStallKeepAlive(t *testing.T) {
	upgrader := websocket.Upgrader{}
	pingReceived := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetPingHandler(func(appData string) error {
			select {
			case pingReceived <- struct{}{}:
			default:
			}
			return conn.WriteControl(
				websocket.PongMessage,
				[]byte(appData),
				time.Now().Add(time.Second),
			)
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL), nil)
	require.NoError(t, err)
	defer conn.Close()

	cli := NewStreamClient(WithAutoReconnect(false))
	cli.conn = conn
	cli.writeTimeout = time.Second

	// Holding writeMutex models an application data write that is blocked while
	// owning the SDK's data-write lock. Keepalive control frames must not need
	// that lock.
	cli.writeMutex.Lock()
	writeLockHeld := true
	defer func() {
		if writeLockHeld {
			cli.writeMutex.Unlock()
		}
	}()

	processDone := make(chan struct{})
	go func() {
		cli.processConnection(conn, 20*time.Millisecond)
		close(processDone)
	}()

	select {
	case <-pingReceived:
	case <-time.After(time.Second):
		t.Fatal("keepalive ping stalled behind the application write lock")
	}

	cli.writeMutex.Unlock()
	writeLockHeld = false
	require.NoError(t, conn.Close())

	select {
	case <-processDone:
	case <-time.After(time.Second):
		t.Fatal("connection processor did not exit after the connection closed")
	}
}

func TestProcessDataFrameResponseUsesSourceConnection(t *testing.T) {
	oldMessages := make(chan string, 1)
	oldConn := newTestWebsocketConn(t, func(conn *websocket.Conn) {
		defer conn.Close()
		_, data, err := conn.ReadMessage()
		if err == nil {
			oldMessages <- string(data)
		}
	})

	newMessages := make(chan string, 1)
	newConn := newTestWebsocketConn(t, func(conn *websocket.Conn) {
		defer conn.Close()
		_, data, err := conn.ReadMessage()
		if err == nil {
			newMessages <- string(data)
		}
	})

	cli := NewStreamClient(WithAutoReconnect(false))
	cli.conn = newConn

	raw := []byte(`{"headers":{"messageId":"source-conn","topic":"ping"},"type":"SYSTEM","data":"{}"}`)
	cli.processDataFrameForConnection(oldConn, raw)

	select {
	case message := <-oldMessages:
		assert.Contains(t, message, "source-conn")
	case <-time.After(time.Second):
		t.Fatal("response was not written to the source connection")
	}

	select {
	case message := <-newMessages:
		t.Fatalf("old frame response leaked to replacement connection: %s", message)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestSendDataFrameResponseUsesContextConnection(t *testing.T) {
	sourceMessages := make(chan string, 1)
	sourceConn := newTestWebsocketConn(t, func(conn *websocket.Conn) {
		defer conn.Close()
		_, data, err := conn.ReadMessage()
		if err == nil {
			sourceMessages <- string(data)
		}
	})

	replacementMessages := make(chan string, 1)
	replacementConn := newTestWebsocketConn(t, func(conn *websocket.Conn) {
		defer conn.Close()
		_, data, err := conn.ReadMessage()
		if err == nil {
			replacementMessages <- string(data)
		}
	})

	cli := NewStreamClient(WithAutoReconnect(false))
	cli.conn = replacementConn
	ctx := context.WithValue(context.Background(), connectionContextKey{}, sourceConn)
	resp := payload.NewSuccessDataFrameResponse()
	resp.SetHeader(payload.DataFrameHeaderKMessageId, "context-source")

	require.NoError(t, cli.SendDataFrameResponse(ctx, resp))

	select {
	case message := <-sourceMessages:
		assert.Contains(t, message, "context-source")
	case <-time.After(time.Second):
		t.Fatal("context-bound response was not written to the source connection")
	}

	select {
	case message := <-replacementMessages:
		t.Fatalf("context-bound response leaked to replacement connection: %s", message)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestOldProcessLoopDoesNotDetachReplacementConnection(t *testing.T) {
	oldConn := newTestWebsocketConn(t, func(conn *websocket.Conn) {
		_ = conn.Close()
	})
	newConn := newTestWebsocketConn(t, func(conn *websocket.Conn) {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})

	cli := NewStreamClient(WithAutoReconnect(false))
	cli.conn = newConn
	cli.sessionId = "replacement"

	done := make(chan struct{})
	go func() {
		cli.processConnection(oldConn, time.Hour)
		close(done)
	}()
	waitProcessLoopExit(t, done, time.Second)

	cli.mutex.Lock()
	currentConn := cli.conn
	sessionID := cli.sessionId
	cli.mutex.Unlock()
	assert.Same(t, newConn, currentConn)
	assert.Equal(t, "replacement", sessionID)
}

func TestCloseCancelsReconnectBackoff(t *testing.T) {
	cli := NewStreamClient(
		WithAppCredential(NewAppCredentialConfig("cid", "csecret")),
		WithAutoReconnect(true),
	)
	lifecycleCtx := cli.lifecycleCtx

	done := make(chan struct{})
	go func() {
		cli.reconnect(lifecycleCtx)
		close(done)
	}()

	cli.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("reconnect backoff was not canceled by Close")
	}
}

func TestDisconnectAckUsesSourceBeforeClosingWithoutClosingReplacement(t *testing.T) {
	ackReceived := make(chan []byte, 1)
	sourceClosed := make(chan struct{})
	oldConn := newTestWebsocketConn(t, func(conn *websocket.Conn) {
		_, message, err := conn.ReadMessage()
		if err == nil {
			ackReceived <- message
		}
		_, _, _ = conn.ReadMessage()
		close(sourceClosed)
	})
	newConn := newTestWebsocketConn(t, func(conn *websocket.Conn) {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})

	cli := NewStreamClient(WithAutoReconnect(false))
	cli.conn = newConn
	cli.sessionId = "replacement"

	raw := []byte(
		`{"headers":{"messageId":"disconnect-1","topic":"disconnect"},"type":"SYSTEM","data":"{}"}`,
	)
	cli.processDataFrameForConnection(oldConn, raw)

	select {
	case ack := <-ackReceived:
		var response payload.DataFrameResponse
		require.NoError(t, json.Unmarshal(ack, &response))
		assert.Equal(t, "disconnect-1", response.GetHeader(payload.DataFrameHeaderKMessageId))
	case <-time.After(time.Second):
		t.Fatal("disconnect ACK was not written to the source connection")
	}
	select {
	case <-sourceClosed:
	case <-time.After(time.Second):
		t.Fatal("source connection was not closed after disconnect ACK")
	}

	cli.mutex.Lock()
	currentConn := cli.conn
	sessionID := cli.sessionId
	cli.mutex.Unlock()
	assert.Same(t, newConn, currentConn)
	assert.Equal(t, "replacement", sessionID)
}

func wsURL(httpURL string) string {
	return "ws" + strings.TrimPrefix(httpURL, "http")
}

func startEchoWSServer(t *testing.T) (endpoint string, closeFn func()) {
	t.Helper()
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	return wsURL(server.URL), server.Close
}

func dialTestWS(t *testing.T) (endpoint string, closeFn func(), conn *websocket.Conn) {
	t.Helper()
	endpoint, closeServer := startEchoWSServer(t)
	c, _, err := websocket.DefaultDialer.Dial(endpoint, nil)
	require.NoError(t, err)
	return endpoint, func() {
		_ = c.Close()
		closeServer()
	}, c
}

func newTestWebsocketConn(t *testing.T, serverHandler func(*websocket.Conn)) *websocket.Conn {
	t.Helper()

	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		go serverHandler(conn)
	}))

	conn, _, err := websocket.DefaultDialer.Dial(wsURL(server.URL), nil)
	if err != nil {
		server.Close()
		t.Fatalf("dial websocket: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
		server.Close()
	})

	return conn
}

func runProcessLoop(t *testing.T, conn *websocket.Conn, keepAliveIdle time.Duration) <-chan struct{} {
	t.Helper()

	cli := &StreamClient{
		conn:          conn,
		AutoReconnect: false,
		keepAliveIdle: keepAliveIdle,
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		cli.processLoop()
	}()
	return done
}

func waitProcessLoopExit(t *testing.T, done <-chan struct{}, timeout time.Duration) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(timeout):
		t.Fatal("processLoop did not exit")
	}
}

func TestDingtalkOpenStreamClient_processLoop_ReadErrorCancelsLoop(t *testing.T) {
	conn := newTestWebsocketConn(t, func(conn *websocket.Conn) {
		_ = conn.Close()
	})

	done := runProcessLoop(t, conn, time.Hour)

	waitProcessLoopExit(t, done, time.Second)
}

func TestDingtalkOpenStreamClient_processLoop_PingTimeoutCancelsLoop(t *testing.T) {
	conn := newTestWebsocketConn(t, func(conn *websocket.Conn) {
		conn.SetPingHandler(func(string) error {
			return nil
		})
		for {
			if _, _, err := conn.NextReader(); err != nil {
				_ = conn.Close()
				return
			}
		}
	})

	done := runProcessLoop(t, conn, 10*time.Millisecond)

	waitProcessLoopExit(t, done, 6*time.Second)
}

func TestDingtalkOpenStreamClient_processLoop_PongHandlerAfterExitDoesNotPanic(t *testing.T) {
	conn := newTestWebsocketConn(t, func(conn *websocket.Conn) {
		_ = conn.Close()
	})

	done := runProcessLoop(t, conn, time.Hour)

	waitProcessLoopExit(t, done, time.Second)

	pongHandler := conn.PongHandler()
	for i := 0; i < 100; i++ {
		func() {
			defer func() {
				if err := recover(); err != nil {
					t.Fatalf("pong handler panicked after processLoop exit: %v", err)
				}
			}()
			if err := pongHandler(""); err != nil {
				t.Fatalf("pong handler returned error: %v", err)
			}
		}()
	}
}
