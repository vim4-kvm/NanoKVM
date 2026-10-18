package middleware

import (
	"net/http/httptest"
	"testing"
)

func TestWebSocketOriginBehindManager(t *testing.T) {
	for _, test := range []struct {
		origin string
		proto  string
		want   bool
	}{
		{"http://board:8080", "http", true},
		{"https://board:8080", "https", true},
		{"http://board:8081", "http", false},
		{"http://other:8080", "http", false},
	} {
		t.Run(test.origin, func(t *testing.T) {
			request := httptest.NewRequest("GET", "http://127.0.0.1:8081/api/ws", nil)
			request.Host = "board:8080"
			request.Header.Set("Origin", test.origin)
			request.Header.Set("X-Forwarded-Proto", test.proto)
			if got := CheckWebSocketOrigin(request); got != test.want {
				t.Fatalf("CheckWebSocketOrigin = %v, want %v", got, test.want)
			}
		})
	}
}
