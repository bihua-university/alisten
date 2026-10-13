package protocol

import (
	"bytes"
	"encoding/json"
	"unicode/utf8"
)

const Version = 1

type RequestSpec struct{ Request, Response string }

// Envelope is shared by requests, responses and room events. Error is present
// only on a failed response; its data is always an empty object.
type Envelope[T any] struct {
	Version   int    `json:"version"`
	Kind      string `json:"kind"`
	Type      string `json:"type"`
	RequestID string `json:"requestId,omitempty"`
	Data      T      `json:"data"`
	Error     *Error `json:"error,omitempty"`
}

type Incoming = Envelope[json.RawMessage]

// DecodeRequest keeps the envelope-level contract (version, kind, type,
// requestId and a known message type); data validation is left to each
// handler, which decodes its own DTO.
func DecodeRequest(raw []byte) (Incoming, *Error) {
	var m Incoming
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return m, &Error{Code: ErrorCodeInvalidMessage, Message: "消息必须是合法的 JSON 对象"}
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return Incoming{}, &Error{Code: ErrorCodeInvalidMessage, Message: "消息必须是合法的 JSON 对象"}
	}
	if m.Version != Version {
		return m, &Error{Code: ErrorCodeUnsupportedVersion, Message: "仅支持 WebSocket 协议 version=1"}
	}
	if m.Kind != "request" || m.Type == "" || !validRequestID(m.RequestID) {
		return m, &Error{Code: ErrorCodeInvalidMessage, Message: "必须提供 kind=request、type 和 requestId（1–128 字符）"}
	}
	if _, ok := Requests[m.Type]; !ok {
		return m, &Error{Code: ErrorCodeUnknownType, Message: "未知的消息 type: " + m.Type}
	}
	return m, nil
}

func validRequestID(id string) bool {
	n := utf8.RuneCountInString(id)
	return n >= 1 && n <= 128
}

func Event[T any](kind string, data T) Envelope[T] {
	return Envelope[T]{Version: Version, Kind: "event", Type: kind, Data: data}
}

func Response[T any](request Incoming, data T) Envelope[T] {
	return Envelope[T]{Version: Version, Kind: "response", Type: request.Type, RequestID: request.RequestID, Data: data}
}

func Failure(request Incoming, err Error) any {
	if _, ok := Requests[request.Type]; !ok || !validRequestID(request.RequestID) {
		return Event(EventProtocolError, err)
	}
	m := Response(request, Empty{})
	m.Error = &err
	return m
}
