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
	"context"
	"go.uber.org/zap"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils/logger"
)

type Manager struct {
	cfg    config.Config
	secret []byte
	logger *zap.SugaredLogger
}

func (m *Manager) AccessToken(ctx context.Context, token string) (*AuthInfo, error) {
	return authenticateByJWT(token, m.secret)
}

func NewTokenManager(cfg config.Config) *Manager {
	bc := cfg.GetBootstrapConfig()
	return &Manager{
		secret: []byte(bc.Encryption.SecretKey),
		cfg:    cfg,
		logger: logger.NewLogger("tokenManager"),
	}
}
