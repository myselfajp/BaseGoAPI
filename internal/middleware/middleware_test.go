package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// newEngine builds a gin engine in test mode with the given middleware attached
// and a single catch-all route that returns 200 "ok" when reached. Trusted
// proxies are cleared so gin derives ClientIP straight from RemoteAddr, making
// the per-IP rate limiter deterministic.
func newEngine(t *testing.T, mw gin.HandlerFunc) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	if err := engine.SetTrustedProxies(nil); err != nil {
		t.Fatalf("SetTrustedProxies(nil) error: %v", err)
	}
	engine.Use(mw)
	engine.Any("/", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return engine
}

// do drives one request through the engine. remoteAddr and origin are applied
// only when non-empty.
func do(engine *gin.Engine, method, remoteAddr, origin string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/", nil)
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestRateLimitBurstThenBlock(t *testing.T) {
	engine := newEngine(t, RateLimit(2))
	const addr = "1.2.3.4:1111"

	// A burst of 2 is allowed for a single IP.
	for i := 1; i <= 2; i++ {
		rec := do(engine, http.MethodGet, addr, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: got status %d, want %d; body=%q", i, rec.Code, http.StatusOK, rec.Body.String())
		}
	}

	// The 3rd request within the window is rejected with 429.
	rec := do(engine, http.MethodGet, addr, "")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("3rd request: got status %d, want %d; body=%q", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}
	if want := "Rate limit exceeded. Please try again later."; !strings.Contains(rec.Body.String(), want) {
		t.Errorf("3rd request body = %q, want it to contain %q", rec.Body.String(), want)
	}
}

func TestRateLimitDisabledAllowsAll(t *testing.T) {
	engine := newEngine(t, RateLimit(0))
	const addr = "5.6.7.8:2222"

	for i := 1; i <= 25; i++ {
		rec := do(engine, http.MethodGet, addr, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d with rate limiting disabled: got status %d, want %d; body=%q", i, rec.Code, http.StatusOK, rec.Body.String())
		}
	}
}

func TestRateLimitIsolatesClientIPs(t *testing.T) {
	engine := newEngine(t, RateLimit(2))
	const addrA = "1.1.1.1:1111"
	const addrB = "2.2.2.2:2222"

	// Exhaust IP A's bucket (burst of 2), then confirm the 3rd is blocked.
	for i := 1; i <= 2; i++ {
		if rec := do(engine, http.MethodGet, addrA, ""); rec.Code != http.StatusOK {
			t.Fatalf("IP A request %d: got status %d, want %d; body=%q", i, rec.Code, http.StatusOK, rec.Body.String())
		}
	}
	if rec := do(engine, http.MethodGet, addrA, ""); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("IP A 3rd request: got status %d, want %d; body=%q", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}

	// IP B has its own untouched bucket and can still burst.
	for i := 1; i <= 2; i++ {
		if rec := do(engine, http.MethodGet, addrB, ""); rec.Code != http.StatusOK {
			t.Fatalf("IP B request %d: got status %d, want %d (buckets should be per-IP); body=%q", i, rec.Code, http.StatusOK, rec.Body.String())
		}
	}
}

func TestCORSAllowOriginHeader(t *testing.T) {
	const allowList = "https://good.example.com,https://also-good.example.com"

	tests := []struct {
		name            string
		origins         string
		reqOrigin       string
		wantAllowOrigin string // "" means the header must be absent
		wantCredentials bool
	}{
		{
			name:            "wildcard echoes star",
			origins:         "*",
			reqOrigin:       "https://anything.example.com",
			wantAllowOrigin: "*",
			wantCredentials: false,
		},
		{
			name:            "allowed origin is echoed with credentials",
			origins:         allowList,
			reqOrigin:       "https://good.example.com",
			wantAllowOrigin: "https://good.example.com",
			wantCredentials: true,
		},
		{
			name:            "disallowed origin gets no allow-origin header",
			origins:         allowList,
			reqOrigin:       "https://evil.example.com",
			wantAllowOrigin: "",
			wantCredentials: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			engine := newEngine(t, CORS(tc.origins))
			rec := do(engine, http.MethodGet, "", tc.reqOrigin)

			if rec.Code != http.StatusOK {
				t.Fatalf("got status %d, want %d; body=%q", rec.Code, http.StatusOK, rec.Body.String())
			}

			gotOrigin := rec.Header().Get("Access-Control-Allow-Origin")
			if gotOrigin != tc.wantAllowOrigin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", gotOrigin, tc.wantAllowOrigin)
			}

			gotCreds := rec.Header().Get("Access-Control-Allow-Credentials")
			wantCreds := ""
			if tc.wantCredentials {
				wantCreds = "true"
			}
			if gotCreds != wantCreds {
				t.Errorf("Access-Control-Allow-Credentials = %q, want %q", gotCreds, wantCreds)
			}
		})
	}
}

func TestCORSPreflightShortCircuits(t *testing.T) {
	engine := newEngine(t, CORS("*"))

	rec := do(engine, http.MethodOptions, "", "https://anything.example.com")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("OPTIONS preflight: got status %d, want %d; body=%q", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	// The route handler would have written "ok"; a short-circuited preflight must not.
	if body := rec.Body.String(); body != "" {
		t.Errorf("OPTIONS preflight body = %q, want empty (handler should be skipped)", body)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("OPTIONS preflight Access-Control-Allow-Origin = %q, want %q", got, "*")
	}
}
