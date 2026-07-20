package auth

import (
	"net/http/httptest"
	"testing"
)

func TestRequestIsSecure(t *testing.T) {
	tests := []struct {
		name           string
		forwardedProto string
		want           bool
	}{
		{name: "plain HTTP", want: false},
		{name: "HTTPS proxy", forwardedProto: "https", want: true},
		{name: "case insensitive", forwardedProto: "HTTPS", want: true},
		{name: "first forwarded value", forwardedProto: "https, http", want: true},
		{name: "HTTP proxy", forwardedProto: "http", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest("GET", "http://open-spanner.test", nil)
			if test.forwardedProto != "" {
				request.Header.Set("X-Forwarded-Proto", test.forwardedProto)
			}
			if got := requestIsSecure(request); got != test.want {
				t.Fatalf("requestIsSecure() = %t, want %t", got, test.want)
			}
		})
	}

	t.Run("direct TLS", func(t *testing.T) {
		request := httptest.NewRequest("GET", "https://open-spanner.test", nil)
		if !requestIsSecure(request) {
			t.Fatal("requestIsSecure() = false, want true")
		}
	})
}
