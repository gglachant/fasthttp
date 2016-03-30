package fasthttp

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Do performs the given http request and fills the given http response.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Response is ignored if resp is nil.
//
// Client determines the server to be requested in the following order:
//
//   - from RequestURI if it contains full url with scheme and host;
//   - from Host header otherwise.
//
// ErrNoFreeConns is returned if all DefaultMaxConnsPerHost connections
// to the requested host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func Do(req *Request, resp *Response) error {
	return defaultClient.Do(req, resp)
}

// DoTimeout performs the given request and waits for response during
// the given timeout duration.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Client determines the server to be requested in the following order:
//
//   - from RequestURI if it contains full url with scheme and host;
//   - from Host header otherwise.
//
// ErrTimeout is returned if the response wasn't returned during
// the given timeout.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func DoTimeout(req *Request, resp *Response, timeout time.Duration) error {
	return defaultClient.DoTimeout(req, resp, timeout)
}

// DoDeadline performs the given request and waits for response until
// the given deadline.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Client determines the server to be requested in the following order:
//
//   - from RequestURI if it contains full url with scheme and host;
//   - from Host header otherwise.
//
// ErrTimeout is returned if the response wasn't returned until
// the given deadline.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func DoDeadline(req *Request, resp *Response, deadline time.Time) error {
	return defaultClient.DoDeadline(req, resp, deadline)
}

// Get appends url contents to dst and returns it as body.
//
// New body buffer is allocated if dst is nil.
func Get(dst []byte, url string) (statusCode int, body []byte, err error) {
	return defaultClient.Get(dst, url)
}

// GetTimeout appends url contents to dst and returns it as body.
//
// New body buffer is allocated if dst is nil.
//
// ErrTimeout error is returned if url contents couldn't be fetched
// during the given timeout.
func GetTimeout(dst []byte, url string, timeout time.Duration) (statusCode int, body []byte, err error) {
	return defaultClient.GetTimeout(dst, url, timeout)
}

// GetDeadline appends url contents to dst and returns it as body.
//
// New body buffer is allocated if dst is nil.
//
// ErrTimeout error is returned if url contents couldn't be fetched
// until the given deadline.
func GetDeadline(dst []byte, url string, deadline time.Time) (statusCode int, body []byte, err error) {
	return defaultClient.GetDeadline(dst, url, deadline)
}

// Post sends POST request to the given url with the given POST arguments.
//
// Response body is appended to dst, which is returned as body.
//
// New body buffer is allocated if dst is nil.
//
// Empty POST body is sent if postArgs is nil.
func Post(dst []byte, url string, postArgs *Args) (statusCode int, body []byte, err error) {
	return defaultClient.Post(dst, url, postArgs)
}

var defaultClient Client

// Client implements http client.
//
// Copying Client by value is prohibited. Create new instance instead.
//
// It is safe calling Client methods from concurrently running goroutines.
type Client struct {
	noCopy noCopy

	// Client name. Used in User-Agent request header.
	//
	// Default client name is used if not set.
	Name string

	// Callback for establishing new connections to hosts.
	//
	// Default Dial is used if not set.
	Dial DialFunc

	// Attempt to connect to both ipv4 and ipv6 addresses if set to true.
	//
	// This option is used only if default TCP dialer is used,
	// i.e. if Dial is blank.
	//
	// By default client connects only to ipv4 addresses,
	// since unfortunately ipv6 remains broken in many networks worldwide :)
	DialDualStack bool

	// TLS config for https connections.
	//
	// Default TLS config is used if not set.
	TLSConfig *tls.Config

	// Maximum number of connections per each host which may be established.
	//
	// DefaultMaxConnsPerHost is used if not set.
	MaxConnsPerHost int

	// Idle keep-alive connections are closed after this duration.
	//
	// By default idle connections are closed
	// after DefaultMaxIdleConnDuration.
	MaxIdleConnDuration time.Duration

	// Per-connection buffer size for responses' reading.
	// This also limits the maximum header size.
	//
	// Default buffer size is used if 0.
	ReadBufferSize int

	// Per-connection buffer size for requests' writing.
	//
	// Default buffer size is used if 0.
	WriteBufferSize int

	// Maximum duration for full response reading (including body).
	//
	// By default response read timeout is unlimited.
	ReadTimeout time.Duration

	// Maximum duration for full request writing (including body).
	//
	// By default request write timeout is unlimited.
	WriteTimeout time.Duration

	// Maximum response body size.
	//
	// The client returns ErrBodyTooLarge if this limit is greater than 0
	// and response body is greater than the limit.
	//
	// By default response body size is unlimited.
	MaxResponseBodySize int

	// Header names are passed as-is without normalization
	// if this option is set.
	//
	// Disabled header names' normalization may be useful only for proxying
	// responses to other clients expecting case-sensitive
	// header names. See https://github.com/valyala/fasthttp/issues/57
	// for details.
	//
	// By default request and response header names are normalized, i.e.
	// The first letter and the first letters following dashes
	// are uppercased, while all the other letters are lowercased.
	// Examples:
	//
	//     * HOST -> Host
	//     * content-type -> Content-Type
	//     * cONTENT-lenGTH -> Content-Length
	DisableHeaderNamesNormalizing bool

	mLock sync.Mutex
	m     map[string]*HostClient
	ms    map[string]*HostClient
}

// Get appends url contents to dst and returns it as body.
//
// New body buffer is allocated if dst is nil.
func (c *Client) Get(dst []byte, url string) (statusCode int, body []byte, err error) {
	return clientGetURL(dst, url, c)
}

// GetTimeout appends url contents to dst and returns it as body.
//
// New body buffer is allocated if dst is nil.
//
// ErrTimeout error is returned if url contents couldn't be fetched
// during the given timeout.
func (c *Client) GetTimeout(dst []byte, url string, timeout time.Duration) (statusCode int, body []byte, err error) {
	return clientGetURLTimeout(dst, url, timeout, c)
}

// GetDeadline appends url contents to dst and returns it as body.
//
// New body buffer is allocated if dst is nil.
//
// ErrTimeout error is returned if url contents couldn't be fetched
// until the given deadline.
func (c *Client) GetDeadline(dst []byte, url string, deadline time.Time) (statusCode int, body []byte, err error) {
	return clientGetURLDeadline(dst, url, deadline, c)
}

// Post sends POST request to the given url with the given POST arguments.
//
// Response body is appended to dst, which is returned as body.
//
// New body buffer is allocated if dst is nil.
//
// Empty POST body is sent if postArgs is nil.
func (c *Client) Post(dst []byte, url string, postArgs *Args) (statusCode int, body []byte, err error) {
	return clientPostURL(dst, url, postArgs, c)
}

// DoTimeout performs the given request and waits for response during
// the given timeout duration.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Client determines the server to be requested in the following order:
//
//   - from RequestURI if it contains full url with scheme and host;
//   - from Host header otherwise.
//
// ErrTimeout is returned if the response wasn't returned during
// the given timeout.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *Client) DoTimeout(req *Request, resp *Response, timeout time.Duration) error {
	return clientDoTimeout(req, resp, timeout, c)
}

// DoDeadline performs the given request and waits for response until
// the given deadline.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Client determines the server to be requested in the following order:
//
//   - from RequestURI if it contains full url with scheme and host;
//   - from Host header otherwise.
//
// ErrTimeout is returned if the response wasn't returned until
// the given deadline.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *Client) DoDeadline(req *Request, resp *Response, deadline time.Time) error {
	return clientDoDeadline(req, resp, deadline, c)
}

// Do performs the given http request and fills the given http response.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Response is ignored if resp is nil.
//
// Client determines the server to be requested in the following order:
//
//   - from RequestURI if it contains full url with scheme and host;
//   - from Host header otherwise.
//
// ErrNoFreeConns is returned if all Client.MaxConnsPerHost connections
// to the requested host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *Client) Do(req *Request, resp *Response) error {
	uri := req.URI()
	host := uri.Host()

	isTLS := false
	scheme := uri.Scheme()
	if bytes.Equal(scheme, strHTTPS) {
		isTLS = true
	} else if !bytes.Equal(scheme, strHTTP) {
		return fmt.Errorf("unsupported protocol %q. http and https are supported", scheme)
	}

	startCleaner := false

	c.mLock.Lock()
	m := c.m
	if isTLS {
		m = c.ms
	}
	if m == nil {
		m = make(map[string]*HostClient)
		if isTLS {
			c.ms = m
		} else {
			c.m = m
		}
	}
	hc := m[string(host)]
	if hc == nil {
		hc = &HostClient{
			Addr:                          addMissingPort(string(host), isTLS),
			Name:                          c.Name,
			Dial:                          c.Dial,
			DialDualStack:                 c.DialDualStack,
			IsTLS:                         isTLS,
			TLSConfig:                     c.TLSConfig,
			MaxConns:                      c.MaxConnsPerHost,
			MaxIdleConnDuration:           c.MaxIdleConnDuration,
			ReadBufferSize:                c.ReadBufferSize,
			WriteBufferSize:               c.WriteBufferSize,
			ReadTimeout:                   c.ReadTimeout,
			WriteTimeout:                  c.WriteTimeout,
			MaxResponseBodySize:           c.MaxResponseBodySize,
			DisableHeaderNamesNormalizing: c.DisableHeaderNamesNormalizing,
		}
		m[string(host)] = hc
		if len(m) == 1 {
			startCleaner = true
		}
	}
	c.mLock.Unlock()

	if startCleaner {
		go c.mCleaner(m)
	}

	return hc.Do(req, resp)
}

func (c *Client) mCleaner(m map[string]*HostClient) {
	mustStop := false
	for {
		t := time.Now()
		c.mLock.Lock()
		for k, v := range m {
			if t.Sub(v.LastUseTime()) > time.Minute {
				delete(m, k)
			}
		}
		if len(m) == 0 {
			mustStop = true
		}
		c.mLock.Unlock()

		if mustStop {
			break
		}
		time.Sleep(10 * time.Second)
	}
}

// DefaultMaxConnsPerHost is the maximum number of concurrent connections
// http client may establish per host by default (i.e. if
// Client.MaxConnsPerHost isn't set).
const DefaultMaxConnsPerHost = 512

// DefaultMaxIdleConnDuration is the default duration before idle keep-alive
// connection is closed.
const DefaultMaxIdleConnDuration = 10 * time.Second

// DialFunc must establish connection to addr.
//
// There is no need in establishing TLS (SSL) connection for https.
// The client automatically converts connection to TLS
// if HostClient.IsTLS is set.
//
// TCP address passed to DialFunc always contains host and port.
// Example TCP addr values:
//
//   - foobar.com:80
//   - foobar.com:443
//   - foobar.com:8080
type DialFunc func(addr string) (net.Conn, error)

// HostClient balances http requests among hosts listed in Addr.
//
// HostClient may be used for balancing load among multiple upstream hosts.
//
// It is forbidden copying HostClient instances. Create new instances instead.
//
// It is safe calling HostClient methods from concurrently running goroutines.
type HostClient struct {
	noCopy noCopy

	// Comma-separated list of upstream HTTP server host addresses,
	// which are passed to Dial in round-robin manner.
	//
	// Each address may contain port if default dialer is used.
	// For example,
	//
	//    - foobar.com:80
	//    - foobar.com:443
	//    - foobar.com:8080
	Addr string

	// Client name. Used in User-Agent request header.
	Name string

	// Callback for establishing new connection to the host.
	//
	// Default Dial is used if not set.
	Dial DialFunc

	// Attempt to connect to both ipv4 and ipv6 host addresses
	// if set to true.
	//
	// This option is used only if default TCP dialer is used,
	// i.e. if Dial is blank.
	//
	// By default client connects only to ipv4 addresses,
	// since unfortunately ipv6 remains broken in many networks worldwide :)
	DialDualStack bool

	// Whether to use TLS (aka SSL or HTTPS) for host connections.
	IsTLS bool

	// Optional TLS config.
	TLSConfig *tls.Config

	// Maximum number of connections which may be established to all hosts
	// listed in Addr.
	//
	// DefaultMaxConnsPerHost is used if not set.
	MaxConns int

	// Keep-alive connections are closed after this duration.
	//
	// By default connection duration is unlimited.
	MaxConnDuration time.Duration

	// Idle keep-alive connections are closed after this duration.
	//
	// By default idle connections are closed
	// after DefaultMaxIdleConnDuration.
	MaxIdleConnDuration time.Duration

	// Per-connection buffer size for responses' reading.
	// This also limits the maximum header size.
	//
	// Default buffer size is used if 0.
	ReadBufferSize int

	// Per-connection buffer size for requests' writing.
	//
	// Default buffer size is used if 0.
	WriteBufferSize int

	// Maximum duration for full response reading (including body).
	//
	// By default response read timeout is unlimited.
	ReadTimeout time.Duration

	// Maximum duration for full request writing (including body).
	//
	// By default request write timeout is unlimited.
	WriteTimeout time.Duration

	// Maximum response body size.
	//
	// The client returns ErrBodyTooLarge if this limit is greater than 0
	// and response body is greater than the limit.
	//
	// By default response body size is unlimited.
	MaxResponseBodySize int

	// Header names are passed as-is without normalization
	// if this option is set.
	//
	// Disabled header names' normalization may be useful only for proxying
	// responses to other clients expecting case-sensitive
	// header names. See https://github.com/valyala/fasthttp/issues/57
	// for details.
	//
	// By default request and response header names are normalized, i.e.
	// The first letter and the first letters following dashes
	// are uppercased, while all the other letters are lowercased.
	// Examples:
	//
	//     * HOST -> Host
	//     * content-type -> Content-Type
	//     * cONTENT-lenGTH -> Content-Length
	DisableHeaderNamesNormalizing bool

	clientName  atomic.Value
	lastUseTime uint32

	connsLock  sync.Mutex
	connsCount int
	conns      []*clientConn

	addrsLock sync.Mutex
	addrs     []string
	addrIdx   uint32

	readerPool sync.Pool
	writerPool sync.Pool
}

type clientConn struct {
	c           net.Conn
	createdTime time.Time
	lastUseTime time.Time
}

var startTimeUnix = time.Now().Unix()

// LastUseTime returns time the client was last used
func (c *HostClient) LastUseTime() time.Time {
	n := atomic.LoadUint32(&c.lastUseTime)
	return time.Unix(startTimeUnix+int64(n), 0)
}

// Get appends url contents to dst and returns it as body.
//
// New body buffer is allocated if dst is nil.
func (c *HostClient) Get(dst []byte, url string) (statusCode int, body []byte, err error) {
	return clientGetURL(dst, url, c)
}

// GetTimeout appends url contents to dst and returns it as body.
//
// New body buffer is allocated if dst is nil.
//
// ErrTimeout error is returned if url contents couldn't be fetched
// during the given timeout.
func (c *HostClient) GetTimeout(dst []byte, url string, timeout time.Duration) (statusCode int, body []byte, err error) {
	return clientGetURLTimeout(dst, url, timeout, c)
}

// GetDeadline appends url contents to dst and returns it as body.
//
// New body buffer is allocated if dst is nil.
//
// ErrTimeout error is returned if url contents couldn't be fetched
// until the given deadline.
func (c *HostClient) GetDeadline(dst []byte, url string, deadline time.Time) (statusCode int, body []byte, err error) {
	return clientGetURLDeadline(dst, url, deadline, c)
}

// Post sends POST request to the given url with the given POST arguments.
//
// Response body is appended to dst, which is returned as body.
//
// New body buffer is allocated if dst is nil.
//
// Empty POST body is sent if postArgs is nil.
func (c *HostClient) Post(dst []byte, url string, postArgs *Args) (statusCode int, body []byte, err error) {
	return clientPostURL(dst, url, postArgs, c)
}

type clientDoer interface {
	Do(req *Request, resp *Response) error
}

func clientGetURL(dst []byte, url string, c clientDoer) (statusCode int, body []byte, err error) {
	req := AcquireRequest()

	statusCode, body, err = doRequestFollowRedirects(req, dst, url, c)

	ReleaseRequest(req)
	return statusCode, body, err
}

func clientGetURLTimeout(dst []byte, url string, timeout time.Duration, c clientDoer) (statusCode int, body []byte, err error) {
	deadline := time.Now().Add(timeout)
	return clientGetURLDeadline(dst, url, deadline, c)
}

func clientGetURLDeadline(dst []byte, url string, deadline time.Time, c clientDoer) (statusCode int, body []byte, err error) {
	var sleepTime time.Duration
	for {
		statusCode, body, err = clientGetURLDeadlineFreeConn(dst, url, deadline, c)
		if err != ErrNoFreeConns {
			return statusCode, body, err
		}
		sleepTime = updateSleepTime(sleepTime, deadline)
		time.Sleep(sleepTime)
	}
}

type clientURLResponse struct {
	statusCode int
	body       []byte
	err        error
}

func clientGetURLDeadlineFreeConn(dst []byte, url string, deadline time.Time, c clientDoer) (statusCode int, body []byte, err error) {
	timeout := -time.Since(deadline)
	if timeout <= 0 {
		return 0, dst, ErrTimeout
	}

	var ch chan clientURLResponse
	chv := clientURLResponseChPool.Get()
	if chv == nil {
		chv = make(chan clientURLResponse, 1)
	}
	ch = chv.(chan clientURLResponse)

	req := AcquireRequest()

	// Note that the request continues execution on ErrTimeout until
	// client-specific ReadTimeout exceeds. This helps limiting load
	// on slow hosts by MaxConns* concurrent requests.
	//
	// Without this 'hack' the load on slow host could exceed MaxConns*
	// concurrent requests, since timed out requests on client side
	// usually continue execution on the host.
	go func() {
		statusCodeCopy, bodyCopy, errCopy := doRequestFollowRedirects(req, dst, url, c)
		ch <- clientURLResponse{
			statusCode: statusCodeCopy,
			body:       bodyCopy,
			err:        errCopy,
		}
	}()

	var tc *time.Timer
	tcv := timerPool.Get()
	if tcv == nil {
		tc = time.NewTimer(timeout)
		tcv = tc
	} else {
		tc = tcv.(*time.Timer)
		initTimer(tc, timeout)
	}

	select {
	case resp := <-ch:
		ReleaseRequest(req)
		clientURLResponseChPool.Put(chv)
		statusCode = resp.statusCode
		body = resp.body
		err = resp.err
	case <-tc.C:
		body = dst
		err = ErrTimeout
	}

	stopTimer(tc)
	timerPool.Put(tcv)

	return statusCode, body, err
}

var clientURLResponseChPool sync.Pool

func clientPostURL(dst []byte, url string, postArgs *Args, c clientDoer) (statusCode int, body []byte, err error) {
	req := AcquireRequest()
	req.Header.SetMethodBytes(strPost)
	req.Header.SetContentTypeBytes(strPostArgsContentType)
	if postArgs != nil {
		postArgs.WriteTo(req.BodyWriter())
	}

	statusCode, body, err = doRequestFollowRedirects(req, dst, url, c)

	ReleaseRequest(req)
	return statusCode, body, err
}

var (
	errMissingLocation  = errors.New("missing Location header for http redirect")
	errTooManyRedirects = errors.New("too many redirects detected when doing the request")
)

const maxRedirectsCount = 16

func doRequestFollowRedirects(req *Request, dst []byte, url string, c clientDoer) (statusCode int, body []byte, err error) {
	resp := AcquireResponse()
	oldBody := resp.body
	resp.body = dst

	redirectsCount := 0
	for {
		req.parsedURI = false
		req.Header.host = req.Header.host[:0]
		req.SetRequestURI(url)

		if err = c.Do(req, resp); err != nil {
			break
		}
		statusCode = resp.Header.StatusCode()
		if statusCode != StatusMovedPermanently && statusCode != StatusFound && statusCode != StatusSeeOther {
			break
		}

		redirectsCount++
		if redirectsCount > maxRedirectsCount {
			err = errTooManyRedirects
			break
		}
		location := resp.Header.peek(strLocation)
		if len(location) == 0 {
			err = errMissingLocation
			break
		}
		url = getRedirectURL(url, location)
	}

	body = resp.body
	resp.body = oldBody
	ReleaseResponse(resp)

	return statusCode, body, err
}

func getRedirectURL(baseURL string, location []byte) string {
	u := AcquireURI()
	u.Update(baseURL)
	u.UpdateBytes(location)
	redirectURL := u.String()
	ReleaseURI(u)
	return redirectURL
}

var (
	requestPool  sync.Pool
	responsePool sync.Pool
)

// AcquireRequest returns an empty Request instance from request pool.
//
// The returned Request instance may be passed to ReleaseRequest when it is
// no longer needed. This allows Request recycling, reduces GC pressure
// and usually improves performance.
func AcquireRequest() *Request {
	v := requestPool.Get()
	if v == nil {
		return &Request{}
	}
	return v.(*Request)
}

// ReleaseRequest returns req acquired via AcquireRequest to request pool.
//
// It is forbidden accessing req and/or its' members after returning
// it to request pool.
func ReleaseRequest(req *Request) {
	req.Reset()
	requestPool.Put(req)
}

// AcquireResponse returns an empty Response instance from response pool.
//
// The returned Response instance may be passed to ReleaseResponse when it is
// no longer needed. This allows Response recycling, reduces GC pressure
// and usually improves performance.
func AcquireResponse() *Response {
	v := responsePool.Get()
	if v == nil {
		return &Response{}
	}
	return v.(*Response)
}

// ReleaseResponse return resp acquired via AcquireResponse to response pool.
//
// It is forbidden accessing resp and/or its' members after returning
// it to response pool.
func ReleaseResponse(resp *Response) {
	resp.Reset()
	responsePool.Put(resp)
}

// DoTimeout performs the given request and waits for response during
// the given timeout duration.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// ErrTimeout is returned if the response wasn't returned during
// the given timeout.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *HostClient) DoTimeout(req *Request, resp *Response, timeout time.Duration) error {
	return clientDoTimeout(req, resp, timeout, c)
}

// DoDeadline performs the given request and waits for response until
// the given deadline.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// ErrTimeout is returned if the response wasn't returned until
// the given deadline.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *HostClient) DoDeadline(req *Request, resp *Response, deadline time.Time) error {
	return clientDoDeadline(req, resp, deadline, c)
}

func clientDoTimeout(req *Request, resp *Response, timeout time.Duration, c clientDoer) error {
	deadline := time.Now().Add(timeout)
	return clientDoDeadline(req, resp, deadline, c)
}

func clientDoDeadline(req *Request, resp *Response, deadline time.Time, c clientDoer) error {
	var sleepTime time.Duration
	for {
		err := clientDoDeadlineFreeConn(req, resp, deadline, c)
		if err != ErrNoFreeConns {
			return err
		}
		sleepTime = updateSleepTime(sleepTime, deadline)
		time.Sleep(sleepTime)
	}
}

func updateSleepTime(prevTime time.Duration, deadline time.Time) time.Duration {
	sleepTime := prevTime * 2
	if sleepTime == 0 {
		sleepTime = (10 + time.Duration(rand.Intn(40))) * time.Millisecond
	}

	remainingTime := deadline.Sub(time.Now())
	if sleepTime >= remainingTime {
		// Just sleep for the remaining time and then time out.
		// This should save CPU time for real work by other goroutines.
		sleepTime = remainingTime + 10*time.Millisecond
		if sleepTime < 0 {
			sleepTime = 10 * time.Millisecond
		}
	}

	return sleepTime
}

func clientDoDeadlineFreeConn(req *Request, resp *Response, deadline time.Time, c clientDoer) error {
	timeout := -time.Since(deadline)
	if timeout <= 0 {
		return ErrTimeout
	}

	var ch chan error
	chv := errorChPool.Get()
	if chv == nil {
		chv = make(chan error, 1)
	}
	ch = chv.(chan error)

	// Make req and resp copies, since on timeout they no longer
	// may be accessed.
	reqCopy := AcquireRequest()
	req.copyToSkipBody(reqCopy)
	swapRequestBody(req, reqCopy)
	respCopy := AcquireResponse()

	// Note that the request continues execution on ErrTimeout until
	// client-specific ReadTimeout exceeds. This helps limiting load
	// on slow hosts by MaxConns* concurrent requests.
	//
	// Without this 'hack' the load on slow host could exceed MaxConns*
	// concurrent requests, since timed out requests on client side
	// usually continue execution on the host.
	go func() {
		ch <- c.Do(reqCopy, respCopy)
	}()

	var tc *time.Timer
	tcv := timerPool.Get()
	if tcv == nil {
		tc = time.NewTimer(timeout)
		tcv = tc
	} else {
		tc = tcv.(*time.Timer)
		initTimer(tc, timeout)
	}

	var err error
	select {
	case err = <-ch:
		respCopy.copyToSkipBody(resp)
		swapResponseBody(resp, respCopy)
		ReleaseResponse(respCopy)
		ReleaseRequest(reqCopy)
		errorChPool.Put(chv)
	case <-tc.C:
		err = ErrTimeout
	}

	stopTimer(tc)
	timerPool.Put(tcv)

	return err
}

var (
	errorChPool sync.Pool
	timerPool   sync.Pool
)

// Do performs the given http request and sets the corresponding response.
//
// Request must contain at least non-zero RequestURI with full url (including
// scheme and host) or non-zero Host header + RequestURI.
//
// Response is ignored if resp is nil.
//
// ErrNoFreeConns is returned if all HostClient.MaxConns connections
// to the host are busy.
//
// It is recommended obtaining req and resp via AcquireRequest
// and AcquireResponse in performance-critical code.
func (c *HostClient) Do(req *Request, resp *Response) error {
	retry, err := c.do(req, resp)
	if err != nil && retry && isIdempotent(req) {
		_, err = c.do(req, resp)
	}
	if err == io.EOF {
		err = ErrConnectionClosed
	}
	return err
}

func isIdempotent(req *Request) bool {
	return req.Header.IsGet() || req.Header.IsHead() || req.Header.IsPut()
}

func (c *HostClient) do(req *Request, resp *Response) (bool, error) {
	if req == nil {
		panic("BUG: req cannot be nil")
	}

	atomic.StoreUint32(&c.lastUseTime, uint32(time.Now().Unix()-startTimeUnix))

	cc, err := c.acquireConn()
	if err != nil {
		return false, err
	}
	conn := cc.c

	if c.WriteTimeout > 0 {
		if err = conn.SetWriteDeadline(time.Now().Add(c.WriteTimeout)); err != nil {
			c.closeConn(cc)
			return true, err
		}
	}

	resetConnection := false
	if c.MaxConnDuration > 0 && time.Since(cc.createdTime) > c.MaxConnDuration && !req.ConnectionClose() {
		req.SetConnectionClose()
		resetConnection = true
	}

	userAgentOld := req.Header.UserAgent()
	if len(userAgentOld) == 0 {
		req.Header.userAgent = c.getClientName()
	}
	bw := c.acquireWriter(conn)
	err = req.Write(bw)
	if len(userAgentOld) == 0 {
		req.Header.userAgent = userAgentOld
	}

	if resetConnection {
		req.Header.ResetConnectionClose()
	}

	if err == nil {
		err = bw.Flush()
	}
	if err != nil {
		c.releaseWriter(bw)
		c.closeConn(cc)
		return true, err
	}
	c.releaseWriter(bw)

	nilResp := false
	if resp == nil {
		nilResp = true
		resp = AcquireResponse()
	}

	if c.ReadTimeout > 0 {
		if err = conn.SetReadDeadline(time.Now().Add(c.ReadTimeout)); err != nil {
			if nilResp {
				ReleaseResponse(resp)
			}
			c.closeConn(cc)
			return true, err
		}
	}

	if !req.Header.IsGet() && req.Header.IsHead() {
		resp.SkipBody = true
	}
	if c.DisableHeaderNamesNormalizing {
		resp.Header.DisableNormalizing()
	}

	br := c.acquireReader(conn)
	if err = resp.ReadLimitBody(br, c.MaxResponseBodySize); err != nil {
		if nilResp {
			ReleaseResponse(resp)
		}
		c.releaseReader(br)
		c.closeConn(cc)
		if err == io.EOF {
			return true, err
		}
		return false, err
	}
	c.releaseReader(br)

	if resetConnection || req.ConnectionClose() || resp.ConnectionClose() {
		c.closeConn(cc)
	} else {
		c.releaseConn(cc)
	}

	if nilResp {
		ReleaseResponse(resp)
	}
	return false, err
}

var (
	// ErrNoFreeConns is returned when no free connections available
	// to the given host.
	ErrNoFreeConns = errors.New("no free connections available to host")

	// ErrTimeout is returned from timed out calls.
	ErrTimeout = errors.New("timeout")

	// ErrConnectionClosed may be returned from client methods if the server
	// closes connection before returning the first response byte.
	//
	// If you see this error, then either fix the server by returning
	// 'Connection: close' response header before closing the connection
	// or add 'Connection: close' request header before sending requests
	// to broken server.
	ErrConnectionClosed = errors.New("the server closed connection before returning the first response byte. " +
		"Make sure the server returns 'Connection: close' response header before closing the connection")
)

func (c *HostClient) acquireConn() (*clientConn, error) {
	var cc *clientConn
	createConn := false
	startCleaner := false

	var n int
	c.connsLock.Lock()
	n = len(c.conns)
	if n == 0 {
		maxConns := c.MaxConns
		if maxConns <= 0 {
			maxConns = DefaultMaxConnsPerHost
		}
		if c.connsCount < maxConns {
			c.connsCount++
			createConn = true
		}
		if createConn && c.connsCount == 1 {
			startCleaner = true
		}
	} else {
		n--
		cc = c.conns[n]
		c.conns = c.conns[:n]
	}
	c.connsLock.Unlock()

	if cc != nil {
		return cc, nil
	}
	if !createConn {
		return nil, ErrNoFreeConns
	}

	conn, err := c.dialHostHard()
	if err != nil {
		c.decConnsCount()
		return nil, err
	}
	cc = acquireClientConn(conn)

	if startCleaner {
		go c.connsCleaner()
	}
	return cc, nil
}

func (c *HostClient) connsCleaner() {
	var (
		scratch             []*clientConn
		mustStop            bool
		maxIdleConnDuration = c.MaxIdleConnDuration
	)
	if maxIdleConnDuration <= 0 {
		maxIdleConnDuration = DefaultMaxIdleConnDuration
	}
	for {
		currentTime := time.Now()

		c.connsLock.Lock()
		conns := c.conns
		n := len(conns)
		i := 0
		for i < n && currentTime.Sub(conns[i].lastUseTime) > maxIdleConnDuration {
			i++
		}
		mustStop = (c.connsCount == i)
		scratch = append(scratch[:0], conns[:i]...)
		if i > 0 {
			m := copy(conns, conns[i:])
			for i = m; i < n; i++ {
				conns[i] = nil
			}
			c.conns = conns[:m]
		}
		c.connsLock.Unlock()

		for i, cc := range scratch {
			c.closeConn(cc)
			scratch[i] = nil
		}
		if mustStop {
			break
		}
		time.Sleep(maxIdleConnDuration)
	}
}

func (c *HostClient) closeConn(cc *clientConn) {
	c.decConnsCount()
	cc.c.Close()
	releaseClientConn(cc)
}

func (c *HostClient) decConnsCount() {
	c.connsLock.Lock()
	c.connsCount--
	c.connsLock.Unlock()
}

func acquireClientConn(conn net.Conn) *clientConn {
	v := clientConnPool.Get()
	if v == nil {
		v = &clientConn{}
	}
	cc := v.(*clientConn)
	cc.c = conn
	cc.createdTime = time.Now()
	return cc
}

func releaseClientConn(cc *clientConn) {
	cc.c = nil
	clientConnPool.Put(cc)
}

var clientConnPool sync.Pool

func (c *HostClient) releaseConn(cc *clientConn) {
	cc.lastUseTime = time.Now()
	c.connsLock.Lock()
	c.conns = append(c.conns, cc)
	c.connsLock.Unlock()
}

func (c *HostClient) acquireWriter(conn net.Conn) *bufio.Writer {
	v := c.writerPool.Get()
	if v == nil {
		n := c.WriteBufferSize
		if n <= 0 {
			n = defaultWriteBufferSize
		}
		return bufio.NewWriterSize(conn, n)
	}
	bw := v.(*bufio.Writer)
	bw.Reset(conn)
	return bw
}

func (c *HostClient) releaseWriter(bw *bufio.Writer) {
	c.writerPool.Put(bw)
}

func (c *HostClient) acquireReader(conn net.Conn) *bufio.Reader {
	v := c.readerPool.Get()
	if v == nil {
		n := c.ReadBufferSize
		if n <= 0 {
			n = defaultReadBufferSize
		}
		return bufio.NewReaderSize(conn, n)
	}
	br := v.(*bufio.Reader)
	br.Reset(conn)
	return br
}

func (c *HostClient) releaseReader(br *bufio.Reader) {
	c.readerPool.Put(br)
}

func newDefaultTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
		ClientSessionCache: tls.NewLRUClientSessionCache(0),
	}
}

func (c *HostClient) nextAddr() string {
	c.addrsLock.Lock()
	if c.addrs == nil {
		c.addrs = strings.Split(c.Addr, ",")
	}
	addr := c.addrs[0]
	if len(c.addrs) > 1 {
		addr = c.addrs[c.addrIdx%uint32(len(c.addrs))]
		c.addrIdx++
	}
	c.addrsLock.Unlock()
	return addr
}

func (c *HostClient) dialHostHard() (conn net.Conn, err error) {
	// attempt to dial all the available hosts before giving up.

	c.addrsLock.Lock()
	n := len(c.addrs)
	c.addrsLock.Unlock()

	if n == 0 {
		// It looks like c.addrs isn't initialized yet.
		n = 1
	}

	timeout := c.ReadTimeout + c.WriteTimeout
	if timeout <= 0 {
		timeout = DefaultDialTimeout
	}
	deadline := time.Now().Add(timeout)
	for n > 0 {
		conn, err = c.dialHost()
		if err == nil {
			return conn, nil
		}
		if time.Since(deadline) >= 0 {
			break
		}
		n--
	}
	return nil, err
}

func (c *HostClient) dialHost() (net.Conn, error) {
	dial := c.Dial
	addr := c.nextAddr()
	if dial == nil {
		if c.DialDualStack {
			dial = DialDualStack
		} else {
			dial = Dial
		}
		addr = addMissingPort(addr, c.IsTLS)
	}
	conn, err := dial(addr)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		panic("BUG: DialFunc returned (nil, nil)")
	}
	if c.IsTLS {
		tlsConfig := c.TLSConfig
		if tlsConfig == nil {
			tlsConfig = newDefaultTLSConfig()
		}
		conn = tls.Client(conn, tlsConfig)
	}
	return conn, nil
}

func (c *HostClient) getClientName() []byte {
	v := c.clientName.Load()
	var clientName []byte
	if v == nil {
		clientName = []byte(c.Name)
		if len(clientName) == 0 {
			clientName = defaultUserAgent
		}
		c.clientName.Store(clientName)
	} else {
		clientName = v.([]byte)
	}
	return clientName
}

func addMissingPort(addr string, isTLS bool) string {
	n := strings.Index(addr, ":")
	if n >= 0 {
		return addr
	}
	port := 80
	if isTLS {
		port = 443
	}
	return fmt.Sprintf("%s:%d", addr, port)
}
