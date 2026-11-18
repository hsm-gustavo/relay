package workspace

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/publicsuffix"
)

// SessionCookies has no persistent representation. Clearing replaces the jar
// so a request in flight cannot repopulate the selected session.
type SessionCookies struct {
	mu   sync.Mutex
	jars map[string]*cookiejar.Jar
}

func (s *SessionCookies) ForRequest(environmentID string) *CookieUse {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.jars == nil {
		s.jars = make(map[string]*cookiejar.Jar)
	}
	jar := s.jars[environmentID]
	if jar == nil {
		// The standard implementation accepts this fixed options value.
		jar, _ = cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
		s.jars[environmentID] = jar
	}
	return &CookieUse{jar: jar}
}

func (s *SessionCookies) Clear(environmentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jars, environmentID)
}

func (s *SessionCookies) Names(environmentID string, u *url.URL) []string {
	s.mu.Lock()
	jar := s.jars[environmentID]
	s.mu.Unlock()
	if jar == nil {
		return nil
	}
	var names []string
	for _, cookie := range jar.Cookies(u) {
		names = append(names, cookie.Name)
	}
	sort.Strings(names)
	return names
}

// CookieUse tracks whether a specific send accessed cookie values, including
// cookies accepted on intermediate redirect responses.
type CookieUse struct {
	jar  *cookiejar.Jar
	used atomic.Bool
}

func (c *CookieUse) Cookies(u *url.URL) []*http.Cookie {
	cookies := c.jar.Cookies(u)
	if len(cookies) > 0 {
		c.used.Store(true)
	}
	return cookies
}
func (c *CookieUse) SetCookies(u *url.URL, cookies []*http.Cookie) {
	if len(cookies) > 0 {
		c.used.Store(true)
	}
	c.jar.SetCookies(u, cookies)
}
func (c *CookieUse) Used() bool { return c.used.Load() }

// Add stores a host-only cookie in the selected session. The cookie jar applies
// path matching, secure transport, and expiry on subsequent requests.
func (s *SessionCookies) Add(environmentID string, target *url.URL, cookie http.Cookie) error {
	if target == nil || target.Hostname() == "" || (target.Scheme != "http" && target.Scheme != "https") {
		return fmt.Errorf("a valid HTTP or HTTPS request URL is required")
	}
	if cookie.Domain != "" {
		return fmt.Errorf("manually added cookies must be scoped to the request host")
	}
	if !strings.HasPrefix(cookie.Path, "/") {
		return fmt.Errorf("cookie path must start with /")
	}
	if cookie.Secure && target.Scheme != "https" {
		return fmt.Errorf("open an HTTPS request to add a Secure cookie")
	}
	if !cookie.Expires.IsZero() && !cookie.Expires.After(time.Now()) {
		return fmt.Errorf("cookie expiry must be in the future")
	}
	if err := cookie.Valid(); err != nil {
		return fmt.Errorf("invalid cookie: %w", err)
	}
	s.ForRequest(environmentID).SetCookies(target, []*http.Cookie{&cookie})
	return nil
}
