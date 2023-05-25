package payload

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
)

/**
 * @Author linya.jj
 * @Date 2023/3/31 09:57
 */

func TestDataFrameHeader_GetSet(t *testing.T) {
	h := &DataFrameHeader{}
	h.Set("k1", "v1")
	h.Set("k2", "v2")

	assert.Equal(t, "v1", h.Get("k1"))
	assert.Equal(t, "v2", h.Get("k2"))
	assert.Equal(t, "", h.Get("k_notexist"))

	h.Set("k1", "v11")
	assert.Equal(t, "v11", h.Get("k1"))
}

func TestDataFrame_EncodeDecode(t *testing.T) {
	df := &DataFrame{
		SpecVersion: "sv",
		Type:        "t",
		Time:        123456,
		Headers:     DataFrameHeader{"k1": "v1"},
		Data:        "data",
	}

	ss := df.Encode()
	assert.NotEqual(t, "", string(ss))

	df0, err := DecodeDataFrame(ss)
	assert.Nil(t, err)
	assert.EqualValues(t, df, df0)

	ss = []byte(`{"time":"time"}`)
	_, err = DecodeDataFrame([]byte(ss))
	assert.NotNil(t, err)
}

func TestDataFrameResponse_EncodeDecode(t *testing.T) {
	resp := &DataFrameResponse{
		Code:    200,
		Headers: DataFrameHeader{"k1": "v1"},
		Message: "msg",
		Data:    "data",
	}

	ss := resp.Encode()
	assert.NotEqual(t, "", string(ss))

	resp0, err := DecodeDataFrameResponse(ss)
	assert.Nil(t, err)
	assert.EqualValues(t, resp, resp0)

	ss = []byte(`{"code":"code"}`)
	_, err = DecodeDataFrameResponse(ss)
	assert.NotNil(t, err)
}

func TestNewDataFrameAckPong(t *testing.T) {
	pong := NewDataFrameAckPong("messageId")
	assert.NotNil(t, pong)
}

func TestNewErrorDataFrameResponse(t *testing.T) {
	errResp := NewErrorDataFrameResponse(errors.New("error"))
	assert.NotNil(t, errResp)
	assert.Equal(t, "error", errResp.Message)
}
