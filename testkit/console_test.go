package testkit

import (
	"testing"

	"github.com/go-rod/rod/lib/proto"
	"github.com/stretchr/testify/assert"
	"github.com/ysmood/gson"
)

func TestSSEDisconnect(t *testing.T) {
	str := func(s string) *proto.RuntimeRemoteObject {
		return &proto.RuntimeRemoteObject{Type: proto.RuntimeRemoteObjectTypeString, Value: gson.New(s)}
	}
	obj := &proto.RuntimeRemoteObject{Description: "TypeError: network error", Type: proto.RuntimeRemoteObjectTypeObject}

	tests := []struct {
		args []*proto.RuntimeRemoteObject
		name string
		want bool
	}{
		{args: []*proto.RuntimeRemoteObject{str("htmx: htmx:sse:error: network error"), obj}, name: "matches a stream cut by navigation", want: true},
		{args: []*proto.RuntimeRemoteObject{str("htmx: htmx:sse:error: SSE reconnect failed with status 500")}, name: "keeps a failed reconnect as an error"},
		{args: []*proto.RuntimeRemoteObject{str("htmx: htmx:error: network error")}, name: "keeps other htmx network errors"},
		{args: []*proto.RuntimeRemoteObject{obj}, name: "ignores calls without a message string"},
		{name: "ignores empty calls"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.New(t).Equal(tt.want, sseDisconnect(tt.args))
		})
	}
}
