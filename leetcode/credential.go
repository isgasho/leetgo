package leetcode

import (
	"errors"
	"net/http"
	"net/url"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/zellyn/kooky"
	_ "github.com/zellyn/kooky/browser/chrome"

	"github.com/j178/leetgo/config"
)

type CredentialsProvider interface {
	AddCredentials(req *http.Request) error
}

type ResettableProvider interface {
	Reset()
}

type NeedClient interface {
	SetClient(c Client)
}

type nonAuth struct{}

func NonAuth() CredentialsProvider {
	return &nonAuth{}
}

func (n *nonAuth) AddCredentials(req *http.Request) error {
	return nil
}

func (n *nonAuth) Reset() {}

type cookiesAuth struct {
	LeetcodeSession string
	CsrfToken       string
}

func NewCookiesAuth(session, csrftoken string) CredentialsProvider {
	return &cookiesAuth{LeetcodeSession: session, CsrfToken: csrftoken}
}

func (c *cookiesAuth) AddCredentials(req *http.Request) error {
	req.AddCookie(&http.Cookie{Name: "LEETCODE_SESSION", Value: c.LeetcodeSession})
	req.AddCookie(&http.Cookie{Name: "csrftoken", Value: c.CsrfToken})
	req.Header.Add("x-csrftoken", c.CsrfToken)
	return nil
}

func (c *cookiesAuth) Reset() {}

func (c *cookiesAuth) hasAuth() bool {
	return c.LeetcodeSession != "" && c.CsrfToken != ""
}

type passwordAuth struct {
	cookiesAuth
	mu       sync.Mutex
	c        Client
	username string
	password string
}

func NewPasswordAuth(username, passwd string) CredentialsProvider {
	return &passwordAuth{username: username, password: passwd}
}

func (p *passwordAuth) SetClient(c Client) {
	p.c = c
}

func (p *passwordAuth) AddCredentials(req *http.Request) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.hasAuth() {
		hclog.L().Info("logging in with username and password")
		resp, err := p.c.Login(p.username, p.password)
		if err != nil {
			return err
		}
		cookies := resp.Cookies()
		for _, cookie := range cookies {
			if cookie.Name == "LEETCODE_SESSION" {
				p.LeetcodeSession = cookie.Value
			}
			if cookie.Name == "csrftoken" {
				p.CsrfToken = cookie.Value
			}
		}
		if !p.hasAuth() {
			return errors.New("no credential found")
		}
	}
	return p.cookiesAuth.AddCredentials(req)
}

func (p *passwordAuth) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.LeetcodeSession = ""
	p.CsrfToken = ""
}

type browserAuth struct {
	cookiesAuth
	mu       sync.Mutex
	browsers []string
	c        Client
}

func NewBrowserAuth(browsers ...string) CredentialsProvider {
	return &browserAuth{browsers: browsers}
}

func (b *browserAuth) SetClient(c Client) {
	b.c = c
}

func (b *browserAuth) AddCredentials(req *http.Request) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.hasAuth() {
		u, _ := url.Parse(b.c.BaseURI())
		domain := u.Host
		hclog.L().Info("reading credentials from browser", "browser", "chrome", "domain", domain)
		session := kooky.ReadCookies(
			kooky.Valid,
			kooky.DomainContains(domain),
			kooky.Name("LEETCODE_SESSION"),
		)
		csrfToken := kooky.ReadCookies(
			kooky.Valid,
			kooky.DomainContains(domain),
			kooky.Name("csrftoken"),
		)
		if len(session) == 0 || len(csrfToken) == 0 {
			return errors.New("no cookie found in browser")
		}
		b.LeetcodeSession = session[0].Value
		b.CsrfToken = csrfToken[0].Value
		hclog.L().Debug("found credentials in browser")
	}

	return b.cookiesAuth.AddCredentials(req)
}

func (b *browserAuth) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.LeetcodeSession = ""
	b.CsrfToken = ""
}

func CredentialsFromConfig() CredentialsProvider {
	cfg := config.Get()
	switch cfg.LeetCode.Credentials.From {
	case "browser":
		return NewBrowserAuth("chrome")
	case "password":
		return NewPasswordAuth(cfg.LeetCode.Credentials.Username, cfg.LeetCode.Credentials.Password)
	case "cookies":
		return NewCookiesAuth(cfg.LeetCode.Credentials.Session, cfg.LeetCode.Credentials.CsrfToken)
	default:
		return NonAuth()
	}
}
