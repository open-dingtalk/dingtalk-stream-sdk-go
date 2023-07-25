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
		panic(errors.New("illegal callback handler, the number of parameters must be 1"))
	}
	if handlerType.NumOut() > 2 {
		panic(errors.New("illegal callback handler, the number of result cannot exceed 2"))
	}
	return &CallbackFrameHandler{callBackHandler: callFunc, parameterType: handlerType.In(0)}
}

type CallbackFrameHandler struct {
	parameterType reflect.Type

	callBackHandler interface{}
}

func (h *CallbackFrameHandler) handle(context context.Context, df *payload.DataFrame) (*payload.DataFrameResponse, error) {
	value, e := h.unMarshaller(df.Data)
	if e != nil {
		return nil, e
	}

	callbackFunc := reflect.ValueOf(h.callBackHandler)
	input := []reflect.Value{reflect.ValueOf(value)}
	invokeResult := callbackFunc.Call(input)

	resultMeta := ParseResultMeta(h.callBackHandler)
	if e = resultMeta.getError(invokeResult); e != nil {
		return nil, e
	}
	response := payload.NewDataFrameResponse(payload.DataFrameResponseStatusCodeKOK)
	callbackPayload := &CallbackPayload{}
	callbackPayload.Response = resultMeta.getResult(invokeResult)
	if e = response.SetJson(callbackPayload); e != nil {
		return nil, e
	}
	return response, nil
}

type ResultMeta struct {
	ErrorIndex int

	ResultIndex int
}

func ParseResultMeta(callbackHandler interface{}) *ResultMeta {
	result := &ResultMeta{ErrorIndex: -1, ResultIndex: -1}
	funcType := reflect.TypeOf(callbackHandler)
	for i := 0; i < funcType.NumOut(); i++ {
		if funcType.Out(i).Kind() == reflect.Interface && funcType.Out(i).String() == "error" {
			result.ErrorIndex = i
		} else {
			result.ResultIndex = i
		}
	}
	return result
}

func (r *ResultMeta) getError(value []reflect.Value) error {
	if r.ErrorIndex >= 0 {
		errorValue := value[r.ErrorIndex]
		if errorValue.IsNil() {
			return nil
		}
		return value[r.ErrorIndex].Interface().(error)
	} else {
		return nil
	}
}

func (r *ResultMeta) getResult(value []reflect.Value) any {
	if r.ResultIndex >= 0 {
		return value[r.ResultIndex].Interface()
	} else {
		return nil
	}
}
