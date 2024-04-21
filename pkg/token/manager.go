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
	"encoding/base64"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"go.uber.org/zap"
	"os"
	"time"
)

type Manager struct {
	store  metastore.AccessToken
	cfg    config.Bootstrap
	logger *zap.SugaredLogger
}

func (m *Manager) AccessToken(ctx context.Context, ak, sk string) (*types.AccessToken, error) {
	token, err := m.store.GetAccessToken(ctx, ak, sk)
	if err != nil {
		return nil, err
	}

	nowTime := time.Now()
	if token.CertExpiration.IsZero() || nowTime.Add(-24*time.Hour).Before(token.CertExpiration) {
		if err = m.resignCerts(ctx, token); err != nil {
			m.logger.Errorw("resign client certs failed", "err", err)
			return nil, err
		}
	}
	return token, nil
}

func (m *Manager) resignCerts(ctx context.Context, token *types.AccessToken) (err error) {
	caCertFile := m.cfg.FsApi.CaFile
	caKeyFile := m.cfg.FsApi.CaKeyFile

	_, err = os.Stat(caCertFile)
	if err != nil {
		return fmt.Errorf("stat ca cert file error: %w", err)
	}

	_, err = os.Stat(caKeyFile)
	if err != nil {
		return fmt.Errorf("stat ca cert file error: %w", err)
	}

	caCertContent, err := os.ReadFile(caCertFile)
	if err != nil {
		return fmt.Errorf("load ca cert file error: %w", err)
	}
	caKeyContent, err := os.ReadFile(caKeyFile)
	if err != nil {
		return fmt.Errorf("load ca cert file error: %w", err)
	}

	rawCert, rawKey, err := (&utils.CertTool{
		CaCertPEM: caCertContent,
		CaKeyPEM:  caKeyContent,
	}).GenerateCertPair(
		token.Namespace, // O
		token.TokenKey,  // OU
		fmt.Sprintf("%d,%d", token.UID, token.GID), // CN
	)
	if err != nil {
		m.logger.Errorw("generate access token certs failed", "err", err)
		return
	}

	token.ClientCrt = base64.StdEncoding.EncodeToString(rawCert)
	token.ClientKey = base64.StdEncoding.EncodeToString(rawKey)
	token.CertExpiration = time.Now().AddDate(0, 11, 0) // 1mon buffer

	err = m.store.UpdateAccessTokenCerts(ctx, token)
	if err != nil {
		m.logger.Errorw("write back access token certs failed", "err", err)
		return
	}

	return nil
}

func NewTokenManager(store metastore.AccessToken, cfg config.Bootstrap) *Manager {
	return &Manager{
		store:  store,
		cfg:    cfg,
		logger: logger.NewLogger("tokenManager"),
	}
}
