package webhooks

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func messageTo(url string) Message {
	m := testMessage()
	m.URL = url
	return m
}

func TestDialControl(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		address string
		blocked bool
	}{
		{"127.0.0.1:443", true},
		{"10.0.0.1:443", true},
		{"172.31.255.255:443", true},
		{"192.168.0.1:443", true},
		{"169.254.169.254:443", true},
		{"100.100.1.57:443", true},
		{"100.127.255.255:443", true},
		{"0.0.0.0:443", true},
		{"0.255.0.1:443", true},
		{"192.0.0.1:443", true},
		{"198.18.0.1:443", true},
		{"198.19.255.255:443", true},
		{"224.0.0.1:443", true},
		{"240.0.0.1:443", true},
		{"255.255.255.255:443", true},
		{"[::1]:443", true},
		{"[::]:443", true},
		{"[fd00::1]:443", true},
		{"[fc00::1]:443", true},
		{"[fe80::1]:443", true},
		{"[fe80::1%lo0]:443", true},
		{"[2606:4700::1111%lo0]:443", true},
		{"[ff02::1]:443", true},
		{"[64:ff9b::808:808]:443", true},
		{"[::ffff:127.0.0.1]:443", true},
		{"[::ffff:10.0.0.1]:443", true},
		{"[::ffff:100.100.1.57]:443", true},
		{"[2002:7f00:1::1]:443", true},
		{"[2002:a00:1::]:443", true},
		{"[fec0::1]:443", true},
		{"[feff:ffff::1]:443", true},
		{"[64:ff9b:1::a00:1]:443", true},
		{"[::7f00:1]:443", true},
		{"[::808:808]:443", true},
		{"192.0.2.1:443", true},
		{"198.51.100.7:443", true},
		{"203.0.113.255:443", true},
		{"[2001:db8::1]:443", true},
		{"[2001:db8:ffff::1]:443", true},
		{"[2001:db9::1]:443", false},
		{"[2003::1]:443", false},
		{"192.0.3.1:443", false},
		{"198.51.101.1:443", false},
		{"203.0.114.1:443", false},
		{"93.184.216.34:80", true},
		{"93.184.216.34:8443", true},
		{"[2606:4700::1111]:22", true},
		{"hooks.example.com:443", true},
		{"not an address", true},
		{"93.184.216.34:443", false},
		{"100.63.255.255:443", false},
		{"100.128.0.0:443", false},
		{"198.20.0.1:443", false},
		{"[2606:4700::1111]:443", false},
		{"[::ffff:93.184.216.34]:443", false},
	} {
		t.Run(tc.address, func(t *testing.T) {
			t.Parallel()
			err := dialControl("tcp", tc.address, nil)
			if got := errors.Is(err, errBlockedAddress); got != tc.blocked || (!tc.blocked && err != nil) {
				t.Errorf("dialControl(%q) = %v, want blocked %v", tc.address, err, tc.blocked)
			}
		})
	}
}

// The Control check rejects each of these before any packet is sent.
func TestClient_Send_BlocksNonPublicTargets(t *testing.T) {
	t.Parallel()
	client := NewClient(time.Now)
	for _, url := range []string{
		"https://127.0.0.1/x",
		"https://10.0.0.1/x",
		"https://169.254.169.254/x",
		"https://100.100.1.57/x",
		"https://[::1]/x",
		"https://[fd00::1]/x",
		"https://[::ffff:127.0.0.1]/x",
		"https://127.0.0.1:8443/x",
		"https://93.184.216.34:8443/x",
		"https://localhost/x",
	} {
		t.Run(url, func(t *testing.T) {
			t.Parallel()
			got := client.Send(t.Context(), messageTo(url))
			if got != (Result{Failure: FailureBlockedAddress}) {
				t.Errorf("Send = %+v, want blocked_address", got)
			}
		})
	}
}

func TestNewTransport_Settings(t *testing.T) {
	t.Parallel()
	tr := newTransport()
	if tr.Proxy != nil || tr.DialContext == nil || !tr.ForceAttemptHTTP2 || tr.MaxIdleConnsPerHost != 2 {
		t.Errorf("transport = %+v", tr)
	}
	if tr.TLSHandshakeTimeout != 3*time.Second || tr.ResponseHeaderTimeout != 5*time.Second {
		t.Errorf("timeouts = %v %v", tr.TLSHandshakeTimeout, tr.ResponseHeaderTimeout)
	}
	if tr.TLSClientConfig != nil && tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("certificate checks are off")
	}
	if got := newClient(tr, time.Now).http.Timeout; got != 5*time.Second {
		t.Errorf("client timeout = %v", got)
	}
}

func tlsReceiver(t *testing.T, handler http.HandlerFunc) (*Client, string) {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)
	return newClient(srv.Client().Transport, time.Now), srv.URL
}

func TestClient_Send_Success(t *testing.T) {
	t.Parallel()
	type received struct{ body, signature, timestamp string }
	got := make(chan received, 1)
	client, url := tlsReceiver(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got <- received{string(b), r.Header.Get(headerSignature), r.Header.Get(headerTimestamp)}
		w.WriteHeader(http.StatusAccepted)
	})
	client.now = func() time.Time { return time.Unix(1795701600, 0) }
	msg := messageTo(url + "/hook")

	res := client.Send(t.Context(), msg)

	if res != (Result{Status: http.StatusAccepted}) || !res.Delivered() {
		t.Fatalf("Send = %+v", res)
	}
	r := <-got
	if r.body != string(msg.Body) || r.timestamp != "1795701600" || r.signature != Sign(testSecret, 1795701600, msg.Body) {
		t.Errorf("receiver saw %+v", r)
	}
}

func TestClient_Send_StatusOutcomes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		status int
		want   FailureReason
	}{
		{200, ""},
		{204, ""},
		{299, ""},
		{304, FailureHTTPStatus},
		{400, FailureHTTPStatus},
		{404, FailureHTTPStatus},
		{410, FailureGone},
		{429, FailureHTTPStatus},
		{500, FailureHTTPStatus},
	} {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			t.Parallel()
			client, url := tlsReceiver(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status) })
			got := client.Send(t.Context(), messageTo(url))
			if got != (Result{Failure: tc.want, Status: tc.status}) || got.Delivered() != (tc.want == "") {
				t.Errorf("Send = %+v", got)
			}
		})
	}
}

func TestClient_Send_DoesNotFollowRedirects(t *testing.T) {
	t.Parallel()
	for _, status := range []int{301, 302, 307, 308} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			t.Parallel()
			var followed atomic.Bool
			client, url := tlsReceiver(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/target" {
					followed.Store(true)
					return
				}
				http.Redirect(w, r, "/target", status)
			})
			got := client.Send(t.Context(), messageTo(url+"/start"))
			if got != (Result{Failure: FailureHTTPStatus, Status: status}) {
				t.Errorf("Send = %+v", got)
			}
			if followed.Load() {
				t.Error("redirect was followed")
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type countingBody struct {
	read   int
	closed bool
}

func (b *countingBody) Read(p []byte) (int, error) { b.read += len(p); return len(p), nil }
func (b *countingBody) Close() error               { b.closed = true; return nil }

func TestClient_Send_ReadsAtMost4KiBOfTheResponse(t *testing.T) {
	t.Parallel()
	body := &countingBody{}
	client := newClient(roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
	}), time.Now)

	if got := client.Send(t.Context(), messageTo("https://hooks.example.com/x")); !got.Delivered() {
		t.Fatalf("Send = %+v", got)
	}
	if body.read != 4096 || !body.closed {
		t.Errorf("read %d bytes, closed %v", body.read, body.closed)
	}
}

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

func TestClient_Send_TransportErrorOutcomes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
		want FailureReason
	}{
		{"dial blocked", &net.OpError{Op: "dial", Err: errBlockedAddress}, FailureBlockedAddress},
		{"net timeout", &net.OpError{Op: "dial", Err: timeoutErr{}}, FailureTimeout},
		{"deadline", context.DeadlineExceeded, FailureTimeout},
		{"certificate", &tls.CertificateVerificationError{Err: errors.New("unknown authority")}, FailureTLS},
		{"not a tls server", tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"}, FailureTLS},
		{"remote alert", &net.OpError{Op: "remote error", Err: errors.New("tls: handshake failure")}, FailureTLS},
		{"local tls error", errors.New("tls: server selected unsupported protocol version"), FailureTLS},
		{"refused", &net.OpError{Op: "dial", Err: errors.New("connection refused")}, FailureConnection},
		{"dns", &net.DNSError{Err: "no such host"}, FailureConnection},
		{"eof", io.ErrUnexpectedEOF, FailureConnection},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client := newClient(roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, tc.err }), time.Now)
			got := client.Send(t.Context(), messageTo("https://hooks.example.com/x"))
			if got != (Result{Failure: tc.want}) {
				t.Errorf("Send = %+v, want %s", got, tc.want)
			}
		})
	}
}

func TestClient_Send_SlowReceiverTimesOut(t *testing.T) {
	t.Parallel()
	release := make(chan struct{})
	client, url := tlsReceiver(t, func(http.ResponseWriter, *http.Request) { <-release })
	t.Cleanup(func() { close(release) })
	client.http.Timeout = 50 * time.Millisecond

	if got := client.Send(t.Context(), messageTo(url)); got != (Result{Failure: FailureTimeout}) {
		t.Errorf("Send = %+v, want timeout", got)
	}
}

func TestClient_Send_UntrustedCertificate(t *testing.T) {
	t.Parallel()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(srv.Close)
	tr := &http.Transport{}
	t.Cleanup(tr.CloseIdleConnections)

	if got := newClient(tr, time.Now).Send(t.Context(), messageTo(srv.URL)); got != (Result{Failure: FailureTLS}) {
		t.Errorf("Send = %+v, want tls", got)
	}
}

func TestClient_Send_UnusableURL(t *testing.T) {
	t.Parallel()
	got := NewClient(time.Now).Send(t.Context(), messageTo("https://bad url/%zz"))
	if got != (Result{Failure: FailureConnection}) {
		t.Errorf("Send = %+v, want connection", got)
	}
}

func spanText(spans []sdktrace.ReadOnlySpan) string {
	var b strings.Builder
	for _, s := range spans {
		b.WriteString(s.Name() + " " + s.Status().Description + "\n")
		for _, kv := range s.Attributes() {
			fmt.Fprintf(&b, "%s=%s\n", kv.Key, kv.Value.String())
		}
		for _, e := range s.Events() {
			fmt.Fprintf(&b, "%s %v\n", e.Name, e.Attributes)
		}
	}
	return b.String()
}

type failingBody struct{ err error }

func (b failingBody) Read([]byte) (int, error) { return 0, b.err }
func (failingBody) Close() error               { return nil }

func TestClient_Send_SpanNeverCarriesURLOrHost(t *testing.T) {
	t.Parallel()
	hostError := errors.New("lookup secret-host.example.com on 10.9.8.7:53: no such host")
	for _, tc := range []struct {
		name string
		base roundTripFunc
	}{
		{"success", func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: http.NoBody}, nil
		}},
		{"rejected", func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 500, Body: http.NoBody}, nil
		}},
		{"transport error naming the host", func(*http.Request) (*http.Response, error) { return nil, hostError }},
		{"response read error naming the host", func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: failingBody{hostError}}, nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := tracetest.NewSpanRecorder()
			tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
			client := newClient(tc.base, time.Now, otelhttp.WithTracerProvider(tp))

			client.Send(t.Context(), messageTo("https://secret-host.example.com/services/SECRET-TOKEN?q=SECRET-QUERY"))

			spans := rec.Ended()
			if len(spans) != 1 || spans[0].Name() != "Webhook delivery" {
				t.Fatalf("spans = %d: %s", len(spans), spanText(spans))
			}
			text := spanText(spans)
			if !strings.Contains(text, "url.full=https://redacted\n") {
				t.Errorf("url.full is not redacted:\n%s", text)
			}
			for _, leak := range []string{"secret-host", "example.com", "SECRET", "/services", "10.9.8.7"} {
				if strings.Contains(text, leak) {
					t.Errorf("span carries %q:\n%s", leak, text)
				}
			}
		})
	}
}
