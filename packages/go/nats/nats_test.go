package nats

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeConn struct {
	published map[string][]byte
	pubErr    error
	resp      *nats.Msg
	reqErr    error
	closed    bool
	lastSubj  string
	lastDur   time.Duration
}

func newFakeConn() *fakeConn {
	return &fakeConn{published: map[string][]byte{}}
}

func (f *fakeConn) Publish(subject string, data []byte) error {
	if f.pubErr != nil {
		return f.pubErr
	}
	f.lastSubj = subject
	cp := make([]byte, len(data))
	copy(cp, data)
	f.published[subject] = cp
	return nil
}

func (f *fakeConn) Request(subject string, data []byte, timeout time.Duration) (*nats.Msg, error) {
	f.lastSubj = subject
	f.lastDur = timeout
	if f.reqErr != nil {
		return nil, f.reqErr
	}
	return f.resp, nil
}

func (f *fakeConn) Close() { f.closed = true }

// -- Connect --

func withConnectStubs(t *testing.T, dial func(string, ...nats.Option) (*nats.Conn, error), attempts int, wait time.Duration) {
	t.Helper()
	origDial := natsDial
	origAttempts := connectAttempts
	origWait := connectRetryWait
	natsDial = dial
	connectAttempts = attempts
	connectRetryWait = wait
	t.Cleanup(func() {
		natsDial = origDial
		connectAttempts = origAttempts
		connectRetryWait = origWait
	})
}

func TestConnectUsesDefaultURL(t *testing.T) {
	var gotURL string
	withConnectStubs(t, func(url string, opts ...nats.Option) (*nats.Conn, error) {
		gotURL = url
		return nil, nil
	}, 1, 0)

	c, err := Connect("")
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, nats.DefaultURL, gotURL)
}

func TestConnectPreservesCustomURL(t *testing.T) {
	var gotURL string
	withConnectStubs(t, func(url string, opts ...nats.Option) (*nats.Conn, error) {
		gotURL = url
		return nil, nil
	}, 1, 0)

	_, err := Connect("nats://nats.internal:4222")
	require.NoError(t, err)
	assert.Equal(t, "nats://nats.internal:4222", gotURL)
}

func TestConnectRetriesUntilSuccess(t *testing.T) {
	calls := 0
	withConnectStubs(t, func(url string, opts ...nats.Option) (*nats.Conn, error) {
		calls++
		if calls < 3 {
			return nil, errors.New("connection refused")
		}
		return nil, nil
	}, 5, 0)

	c, err := Connect("nats://x:4222")
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, 3, calls)
}

func TestConnectFailsAfterAllAttempts(t *testing.T) {
	calls := 0
	withConnectStubs(t, func(url string, opts ...nats.Option) (*nats.Conn, error) {
		calls++
		return nil, errors.New("nope")
	}, 4, 0)

	c, err := Connect("nats://x:4222")
	require.Error(t, err)
	assert.Nil(t, c)
	assert.Equal(t, 4, calls)
	assert.Contains(t, err.Error(), "failed to connect to NATS after 4 attempts")
	assert.Contains(t, err.Error(), "nope")
}

// -- lifecycle --

func TestCloseClosesUnderlyingConnection(t *testing.T) {
	f := newFakeConn()
	newClient(f).Close()
	assert.True(t, f.closed)
}

func TestCloseWithNilConnectionIsSafe(t *testing.T) {
	assert.NotPanics(t, func() { (&NatsClient{}).Close() })
}

func TestConnReturnsNilForTestClient(t *testing.T) {
	assert.Nil(t, newClient(newFakeConn()).Conn())
}

// -- Publish --

func TestPublishSerialisesPayload(t *testing.T) {
	f := newFakeConn()
	c := newClient(f)

	require.NoError(t, c.Publish("orders.created", map[string]string{"id": "1"}))

	assert.Equal(t, "orders.created", f.lastSubj)
	var decoded map[string]string
	require.NoError(t, json.Unmarshal(f.published["orders.created"], &decoded))
	assert.Equal(t, "1", decoded["id"])
}

func TestPublishMarshalFailure(t *testing.T) {
	c := newClient(newFakeConn())

	err := c.Publish("s", make(chan int))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal publish payload")
}

func TestPublishTransportFailure(t *testing.T) {
	f := newFakeConn()
	f.pubErr = errors.New("nats: connection lost")
	c := newClient(f)

	err := c.Publish("s", nil)
	require.Error(t, err)
	assert.Equal(t, "nats: connection lost", err.Error())
}

// -- Request --

func TestRequestReturnsRawResponse(t *testing.T) {
	f := newFakeConn()
	f.resp = &nats.Msg{Data: []byte(`{"ok":true}`)}
	c := newClient(f)

	out, err := c.Request("rpc.get", map[string]int{"a": 1}, 3*time.Second)
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(out))
	assert.Equal(t, "rpc.get", f.lastSubj)
	assert.Equal(t, 3*time.Second, f.lastDur)
}

func TestRequestMarshalFailure(t *testing.T) {
	c := newClient(newFakeConn())

	_, err := c.Request("s", make(chan int), time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal request payload")
}

func TestRequestTransportFailure(t *testing.T) {
	f := newFakeConn()
	f.reqErr = errors.New("nats: request timeout")

	_, err := newClient(f).Request("s", nil, time.Second)
	require.Error(t, err)
	assert.Equal(t, "nats: request timeout", err.Error())
}

// -- NestJS helpers --

func TestPublishNestJSWrapsEnvelope(t *testing.T) {
	f := newFakeConn()
	c := newClient(f)

	require.NoError(t, c.PublishNestJS("order.create", map[string]string{"x": "y"}))

	var env NestJSRequest
	require.NoError(t, json.Unmarshal(f.published["order.create"], &env))
	assert.Equal(t, "order.create", env.Pattern)
	data, ok := env.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "y", data["x"])
	assert.NotEmpty(t, env.ID)
}

func TestPublishNestJSMarshalFailure(t *testing.T) {
	c := newClient(newFakeConn())

	err := c.PublishNestJS("p", make(chan int))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal NestJS publish envelope")
}

func TestPublishNestJSTransportFailure(t *testing.T) {
	f := newFakeConn()
	f.pubErr = errors.New("closed")

	err := newClient(f).PublishNestJS("p", nil)
	require.Error(t, err)
	assert.Equal(t, "closed", err.Error())
}

func TestRequestNestJSSuccess(t *testing.T) {
	f := newFakeConn()
	f.resp = &nats.Msg{Data: []byte(`{"response":{"id":"42"},"id":"1"}`)}
	c := newClient(f)

	out, err := c.RequestNestJS("order.find", map[string]string{"q": "a"}, time.Second)
	require.NoError(t, err)
	assert.JSONEq(t, `{"id":"42"}`, string(out))
	assert.Equal(t, "order.find", f.lastSubj)
}

func TestRequestNestJSMarshalFailure(t *testing.T) {
	c := newClient(newFakeConn())

	_, err := c.RequestNestJS("p", make(chan int), time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal NestJS request envelope")
}

func TestRequestNestJSTransportFailure(t *testing.T) {
	f := newFakeConn()
	f.reqErr = errors.New("no responders")

	_, err := newClient(f).RequestNestJS("p", nil, time.Second)
	require.Error(t, err)
	assert.Equal(t, "no responders", err.Error())
}

func TestRequestNestJSBadResponseEnvelope(t *testing.T) {
	f := newFakeConn()
	f.resp = &nats.Msg{Data: []byte("not-json")}

	_, err := newClient(f).RequestNestJS("p", nil, time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal NestJS response envelope")
}

func TestRequestNestJSRpcError(t *testing.T) {
	f := newFakeConn()
	f.resp = &nats.Msg{Data: []byte(`{"err":{"message":"boom"}}`)}

	_, err := newClient(f).RequestNestJS("p", nil, time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NestJS RPC error")
}
