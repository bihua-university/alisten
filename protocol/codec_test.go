package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeRequestContract(t *testing.T) {
	valid := []string{
		`{"version":1,"kind":"request","type":"heartbeat","requestId":"r1","data":{"clientTimeMs":0}}`,
		`{"version":1,"kind":"request","type":"music.pick","requestId":"r2","data":{"id":"42","source":"netease"}}`,
		`{"version":1,"kind":"request","type":"music.pick","requestId":"r3","data":{"name":"song","source":"qq"}}`,
		`{"version":1,"kind":"request","type":"queue.remove","requestId":"r4","data":{"id":"42","source":"qq"}}`,
		`{"version":1,"kind":"request","type":"music.search","requestId":"r5","data":{"source":"netease","keyword":"x","pageIndex":1,"pageSize":100}}`,
	}
	for _, raw := range valid {
		if _, err := DecodeRequest([]byte(raw)); err != nil {
			t.Errorf("valid request: %s: %+v", raw, err)
		}
	}
	tests := []struct {
		name, raw string
		code      ErrorCode
	}{
		{"null", `null`, ErrorCodeInvalidMessage},
		{"array", `[]`, ErrorCodeInvalidMessage},
		{"invalid JSON", `{`, ErrorCodeInvalidMessage},
		{"missing version", `{"action":"/heartbeat","data":{}}`, ErrorCodeUnsupportedVersion},
		{"unknown version", `{"version":2,"kind":"request","type":"settings.get","requestId":"r","data":{}}`, ErrorCodeUnsupportedVersion},
		{"unknown type", `{"version":1,"kind":"request","type":"new.action","requestId":"r","data":{}}`, ErrorCodeUnknownType},
		{"missing id", `{"version":1,"kind":"request","type":"settings.get","data":{}}`, ErrorCodeInvalidMessage},
		{"event direction", `{"version":1,"kind":"event","type":"settings.get","requestId":"r","data":{}}`, ErrorCodeInvalidMessage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, err := DecodeRequest([]byte(tc.raw))
			if err == nil || err.Code != tc.code {
				t.Fatalf("error = %+v, want %s", err, tc.code)
			}
			if _, marshalErr := json.Marshal(Failure(m, *err)); marshalErr != nil {
				t.Fatal(marshalErr)
			}
		})
	}
	unicodeID := `{"version":1,"kind":"request","type":"settings.get","requestId":"` + strings.Repeat("中", 128) + `","data":{}}`
	if _, err := DecodeRequest([]byte(unicodeID)); err != nil {
		t.Fatalf("128 Unicode characters should be allowed: %+v", err)
	}
}

func TestFailureCorrelation(t *testing.T) {
	err := Error{Code: ErrorCodeInvalidArgument, Message: "boom"}

	correlated := Failure(Incoming{Version: Version, Type: RequestHeartbeat, RequestID: "r1"}, err)
	raw, marshalErr := json.Marshal(correlated)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	var frame Envelope[Empty]
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatal(err)
	}
	if frame.Kind != "response" || frame.Type != RequestHeartbeat || frame.RequestID != "r1" || frame.Error == nil {
		t.Fatalf("correlated failure = %+v (%s)", frame, raw)
	}

	for _, bad := range []Incoming{
		{Version: Version, Type: "new.action", RequestID: "r1"},
		{Version: Version, Type: RequestHeartbeat},
		{Version: Version, Type: RequestHeartbeat, RequestID: strings.Repeat("x", 129)},
	} {
		fallback := Event(EventProtocolError, err)
		raw, err := json.Marshal(Failure(bad, err))
		if err != nil {
			t.Fatal(err)
		}
		want, _ := json.Marshal(fallback)
		if string(raw) != string(want) {
			t.Fatalf("uncorrelatable request %+v must fall back to protocol.error, got %s", bad, raw)
		}
	}
}
