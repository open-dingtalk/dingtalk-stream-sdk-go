package payload

import (
	"encoding/json"
	"strconv"
)

/**
 * @Author linya.jj
 * @Date 2023/3/31 09:57
 */

type DataFrameHeader map[string]string

func (h DataFrameHeader) Get(key string) string {
	return h[key]
}

func (h DataFrameHeader) Set(key, value string) {
	h[key] = value
}

type DataFrame struct {
	SpecVersion string          `json:"specVersion"`
	Type        string          `json:"type"`
	Time        int64           `json:"time"`
	Headers     DataFrameHeader `json:"headers"`
	Data        string          `json:"data"`
}

func (df *DataFrame) Encode() []byte {
	if df == nil {
		return nil
	}

	data, _ := json.Marshal(df)
	return data
}

func (df *DataFrame) GetTopic() string {
	if df == nil {
		return ""
	}

	return df.Headers.Get(DataFrameHeaderKTopic)
}

func (df *DataFrame) GetMessageId() string {
	if df == nil {
		return ""
	}

	return df.Headers.Get(DataFrameHeaderKMessageId)
}

func (df *DataFrame) GetTimestamp() int64 {
	if df == nil {
		return 0
	}

	strTs := df.Headers.Get(DataFrameHeaderKTime)
	ts, err := strconv.ParseInt(strTs, 10, 64)
	if err != nil {
		return 0
	}
	return ts
}

func (df *DataFrame) GetHeader(header string) string {
	if df == nil {
		return ""
	}

	return df.Headers.Get(header)
}

func DecodeDataFrame(rawData []byte) (*DataFrame, error) {
	df := &DataFrame{}

	err := json.Unmarshal(rawData, df)
	if err != nil {
		return nil, err
	}

	return df, nil
}

type DataFrameResponse struct {
	Code    int             `json:"code"`
	Headers DataFrameHeader `json:"headers"`
	Message string          `json:"message"`
	Data    string          `json:"data"`
}

func (df *DataFrameResponse) Encode() []byte {
	if df == nil {
		return nil
	}

	data, _ := json.Marshal(df)
	return data
}

func DecodeDataFrameResponse(rawData []byte) (*DataFrameResponse, error) {
	resp := &DataFrameResponse{}

	err := json.Unmarshal(rawData, resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func NewDataFrameAckPong(messageId string) *DataFrameResponse {
	return &DataFrameResponse{
		Code: DataFrameResponseStatusCodeKOK,
		Headers: DataFrameHeader{
			DataFrameHeaderKContentType: DataFrameContentTypeKJson,
			DataFrameHeaderKMessageId:   messageId,
		},
		Message: "ok",
		Data:    "", //TODO data内容暂留空
	}
}

func NewErrorDataFrameResponse(messageId string, err error) *DataFrameResponse {
	if err == nil {
		return nil
	}

	return &DataFrameResponse{
		Code: 400, //TODO errorcode 细化
		Headers: DataFrameHeader{
			DataFrameHeaderKContentType: DataFrameContentTypeKJson,
			DataFrameHeaderKMessageId:   messageId,
		},
		Message: err.Error(),
		Data:    "", //TODO data内容暂留空
	}
}
