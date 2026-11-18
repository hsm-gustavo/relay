package httpclient

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// GuardRedirect keeps custom headers on their original origin. Go already
// applies its own forwarding policy to standard sensitive headers.
func GuardRedirect(client *http.Client, custom []HeaderField) {
	previous := client.CheckRedirect
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		for _, prior := range via {
			if !sameOrigin(prior.URL, next.URL) {
				for _, row := range custom {
					if !row.Disabled {
						next.Header.Del(row.Name)
					}
				}
				for _, name := range []string{"Authorization", "Cookie", "Cookie2", "Proxy-Authorization"} {
					next.Header.Del(name)
				}
				break
			}
		}
		if previous != nil {
			return previous(next, via)
		}
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return nil
	}
}

func sameOrigin(a, b *url.URL) bool {
	port := func(u *url.URL) string {
		if p := u.Port(); p != "" {
			return p
		}
		if u.Scheme == "https" {
			return "443"
		}
		return "80"
	}
	return a.Scheme == b.Scheme && strings.EqualFold(a.Hostname(), b.Hostname()) && port(a) == port(b)
}
