package clientV2

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/handler"
	"github.com/open-dingtalk/dingtalk-stream-sdk-go/payload"
	"reflect"
)

type CallbackPayload struct {
	Response any `json:"response"`
}

func CallbackFacade(callbackFunc interface{}) handler.IFrameHandler {
	c := BuildFrameHandler(callbackFunc)
	return c.handle
}

func (h *CallbackFrameHandler) unMarshaller(data string) (any, error) {
	pointer := h.newInstance()
	if e := json.Unmarshal([]byte(data), pointer); e != nil {
		return nil, e
	} else {
		if h.parameterType.Kind() == reflect.Pointer {
			return pointer, nil
		} else {
			return reflect.ValueOf(pointer).Elem().Interface(), nil
		}
	}
}

func (h *CallbackFrameHandler) newInstance() any {
	var pointerValue any
	if h.parameterType.Kind() == reflect.Pointer {
		pointerValue = reflect.New(h.parameterType.Elem()).Interface()
	} else if h.parameterType.Kind() == reflect.Struct {
		pointerValue = reflect.New(h.parameterType).Interface()
	} else {
		var data interface{}
		pointerValue = &data
	}
	return pointerValue
}

func BuildFrameHandler(callFunc interface{}) *CallbackFrameHandler {
	if reflect.ValueOf(callFunc).Kind() != reflect.Func {
		panic(errors.New("callback handler must be an function"))
	}

	handlerType := reflect.TypeOf(callFunc)
	if handlerType.NumIn() != 1 {
		panic(errors.New("illegal callback handler"))
	}
	return &CallbackFrameHandler{callBackHandler: callFunc, parameterType: handlerType.In(0)}
}

type CallbackFrameHandler struct {
	parameterType reflect.Type

	callBackHandler interface{}
}

func (h *CallbackFrameHandler) handle(c context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
	value, e := h.unMarshaller(df.Data)
	if e != nil {
		return nil, e
	}

	callback := reflect.ValueOf(h.callBackHandler)
	input := []reflect.Value{reflect.ValueOf(value)}
	result := callback.Call(input)
	if !result[1].IsNil() {
		return nil, result[1].Interface().(error)
	}
	response := payload.NewDataFrameResponse(payload.DataFrameResponseStatusCodeKOK)
	callbackPayload := &CallbackPayload{}
	callbackPayload.Response = result[0].Interface()
	if e = response.SetJson(callbackPayload); e != nil {
		return nil, e
	}
	return response, nil
}
