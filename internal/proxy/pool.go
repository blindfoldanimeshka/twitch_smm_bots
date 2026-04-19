package proxy

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type Proxy struct {
	ID         string
	Host       string
	Port       int
	Username   string
	Password   string
	Protocol   string
	Enabled    bool
	LastUsedAt *time.Time
	FailCount  int
	CreatedAt  time.Time
}

func (p *Proxy) URL() string {
	if p.Username != "" && p.Password != "" {
		return fmt.Sprintf("%s://%s:%s@%s:%d", p.Protocol, p.Username, p.Password, p.Host, p.Port)
	}
	return fmt.Sprintf("%s://%s:%d", p.Protocol, p.Host, p.Port)
}

type Store interface {
	ListProxies(ctx context.Context) ([]Proxy, error)
	GetProxy(ctx context.Context, id string) (*Proxy, error)
	CreateProxy(ctx context.Context, proxy Proxy) error
	UpdateProxyFailCount(ctx context.Context, id string, failCount int) error
	UpdateProxyLastUsed(ctx context.Context, id string, lastUsed time.Time) error
	SetProxyEnabled(ctx context.Context, id string, enabled bool) error
	DeleteProxy(ctx context.Context, id string) error
}

type Pool struct {
	mu      sync.RWMutex
	store   Store
	proxies []Proxy
	current int
}

func NewPool(store Store) *Pool {
	return &Pool{
		store:   store,
		proxies: make([]Proxy, 0),
		current: 0,
	}
}

func (p *Pool) Load(ctx context.Context) error {
	proxies, err := p.store.ListProxies(ctx)
	if err != nil {
		return fmt.Errorf("load proxies: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.proxies = make([]Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		if proxy.Enabled && proxy.FailCount < 3 {
			p.proxies = append(p.proxies, proxy)
		}
	}

	return nil
}

func (p *Pool) Next() (*Proxy, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.proxies) == 0 {
		return nil, fmt.Errorf("no available proxies")
	}

	proxy := &p.proxies[p.current]
	p.current = (p.current + 1) % len(p.proxies)

	return proxy, nil
}

func (p *Pool) Get(id string) (*Proxy, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for i := range p.proxies {
		if p.proxies[i].ID == id {
			return &p.proxies[i], nil
		}
	}

	return nil, fmt.Errorf("proxy not found: %s", id)
}

func (p *Pool) MarkUsed(ctx context.Context, id string) error {
	now := time.Now()
	return p.store.UpdateProxyLastUsed(ctx, id, now)
}

func (p *Pool) MarkFailed(ctx context.Context, id string) error {
	proxy, err := p.Get(id)
	if err != nil {
		return err
	}

	newFailCount := proxy.FailCount + 1
	if err := p.store.UpdateProxyFailCount(ctx, id, newFailCount); err != nil {
		return err
	}

	if newFailCount >= 3 {
		if err := p.store.SetProxyEnabled(ctx, id, false); err != nil {
			return err
		}

		p.mu.Lock()
		for i := range p.proxies {
			if p.proxies[i].ID == id {
				p.proxies = append(p.proxies[:i], p.proxies[i+1:]...)
				if p.current >= len(p.proxies) && len(p.proxies) > 0 {
					p.current = 0
				}
				break
			}
		}
		p.mu.Unlock()
	}

	return nil
}

func (p *Pool) Count() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.proxies)
}
