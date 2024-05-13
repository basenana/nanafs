/*
 Copyright 2023 NanaFS Authors.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package token

import (
	"github.com/basenana/nanafs/pkg/types"
	"sync"
	"time"
)

var (
	defaultCacheTime = time.Minute * 10
	defaultGCTime    = time.Minute * 30
)

type cache struct {
	tokens map[string]*cachedToken
	mux    sync.Mutex
}

func newTokenCache() *cache {
	c := &cache{tokens: map[string]*cachedToken{}}
	go c.gc()
	return c
}

func (c *cache) GetToken(ak, sk string) (*types.AccessToken, error) {
	c.mux.Lock()
	token, ok := c.tokens[ak]
	c.mux.Unlock()
	if !ok {
		return nil, types.ErrNotFound
	}

	if time.Since(token.cacheAt) > defaultCacheTime {
		c.mux.Lock()
		delete(c.tokens, ak)
		c.mux.Unlock()
		return nil, types.ErrNotFound
	}

	if token.AccessToken == nil || token.SecretToken != sk {
		return nil, types.ErrNoAccess
	}
	return token.AccessToken, nil
}

func (c *cache) SetToken(ak string, token *types.AccessToken) {
	c.mux.Lock()
	c.tokens[ak] = &cachedToken{
		AccessToken: token,
		cacheAt:     time.Now(),
	}
	c.mux.Unlock()
}

func (c *cache) gc() {
	gcTimer := time.NewTimer(defaultGCTime)
	for {
		select {
		case <-gcTimer.C:
			c.mux.Lock()
			for k, t := range c.tokens {
				if time.Since(t.cacheAt) > defaultGCTime {
					delete(c.tokens, k)
				}
			}
			c.mux.Unlock()
		}
	}
}

type cachedToken struct {
	*types.AccessToken
	cacheAt time.Time
}
