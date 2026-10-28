// Package valkey keeps the limit windows in Valkey, where every server process sees the same counts.
package valkey

import (
	"context"
	"strconv"
	"time"

	vk "github.com/valkey-io/valkey-go"
)

// add counts an event and opens the key's window with the first one, in one step: a window can never be left
// without its expiry.
var add = vk.NewLuaScript(`local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[1]) end
return n`)

// undo takes one event back only from a window that is still open, so it never leaves a key without its expiry.
var undo = vk.NewLuaScript(`if redis.call('DECR', KEYS[1]) <= 0 then redis.call('DEL', KEYS[1]) end
return 0`)

// Counters is app.Counters on a Valkey server; every key starts with prefix.
type Counters struct {
	client vk.Client
	prefix string
}

// Open connects to the server at url (redis://[user:password@]host:port[/db]) and checks that it answers.
func Open(ctx context.Context, url, prefix string) (*Counters, error) {
	opt, err := vk.ParseURL(url)
	if err != nil {
		return nil, err
	}
	opt.DisableCache = true
	client, err := vk.NewClient(opt)
	if err != nil {
		return nil, err
	}
	if err := client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
		client.Close()
		return nil, err
	}
	return &Counters{client: client, prefix: prefix}, nil
}

func (c *Counters) Close() { c.client.Close() }

func (c *Counters) Add(ctx context.Context, key string, window time.Duration) (int, error) {
	n, err := add.Exec(ctx, c.client, []string{c.prefix + key}, []string{strconv.FormatInt(window.Milliseconds(), 10)}).AsInt64()
	return int(n), err
}

func (c *Counters) Undo(ctx context.Context, key string) error {
	return undo.Exec(ctx, c.client, []string{c.prefix + key}, nil).Error()
}

func (c *Counters) Clear(ctx context.Context, key string) error {
	return c.client.Do(ctx, c.client.B().Del().Key(c.prefix+key).Build()).Error()
}
