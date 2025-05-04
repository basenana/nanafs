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
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
)

type Manager struct {
	store  metastore.AccessToken
	cache  *cache
	cfg    config.Config
	logger *zap.SugaredLogger
}

func (m *Manager) InitBuildinCA(ctx context.Context) error {
	_, err1 := m.cfg.GetSystemConfig(ctx, config.AuthConfigGroup, "ca_cert_0").String()
	_, err2 := m.cfg.GetSystemConfig(ctx, config.AuthConfigGroup, "ca_key_0").String()
	if err1 == nil && err2 == nil {
		return nil
	}

	if !errors.Is(err1, config.ErrNotConfigured) || !errors.Is(err2, config.ErrNotConfigured) {
		return fmt.Errorf("get ca cert/key content failed: %s %s", err1, err2)
	}

	m.logger.Infow("init build-in ca")
	ct := &utils.CertTool{}
	caCert, caKey, err := ct.GenerateCAPair()
	if err != nil {
		return fmt.Errorf("generate new ca pair error: %w", err)
	}

	err = m.cfg.SetSystemConfig(context.Background(), config.AuthConfigGroup, "ca_cert_0", base64.StdEncoding.EncodeToString(caCert))
	if err != nil {
		return fmt.Errorf("writeback ca cert failed: %w", err)
	}
	err = m.cfg.SetSystemConfig(context.Background(), config.AuthConfigGroup, "ca_key_0", base64.StdEncoding.EncodeToString(caKey))
	if err != nil {
		return fmt.Errorf("writeback ca key failed: %w", err)
	}

	return nil
}

func (m *Manager) AccessToken(ctx context.Context, ak, sk string) (*types.AccessToken, error) {
	token, err := m.cache.GetToken(ak, sk)
	if err == nil {
		return token, nil
	}

	if errors.Is(err, types.ErrNoAccess) {
		return nil, err
	}

	token, err = m.store.GetAccessToken(ctx, ak, sk)
	if err != nil {
		if errors.Is(err, types.ErrNotFound) {
			m.cache.SetToken(ak, nil)
			return nil, types.ErrNoAccess
		}
		return nil, err
	}

	nowTime := time.Now()
	if token.CertExpiration.IsZero() || token.CertExpiration.Before(nowTime.Add(24*time.Hour)) {
		if err = m.resignCerts(ctx, token.Namespace, token); err != nil {
			m.logger.Errorw("resign client certs failed", "err", err)
			return nil, err
		}
	}
	m.logger.Infow("access token", "tokenKey", ak)
	m.cache.SetToken(ak, token)
	return token, nil
}

func (m *Manager) resignCerts(ctx context.Context, namespace string, token *types.AccessToken) (err error) {
	caCertEncodedContent, err := m.cfg.GetSystemConfig(ctx, config.AuthConfigGroup, "ca_cert_0").String()
	if err != nil {
		return fmt.Errorf("get ca cert content failed: %w", err)
	}
	caKeyEncodedContent, err := m.cfg.GetSystemConfig(ctx, config.AuthConfigGroup, "ca_key_0").String()
	if err != nil {
		return fmt.Errorf("get ca key content failed: %w", err)
	}

	caCertContent, err := base64.StdEncoding.DecodeString(caCertEncodedContent)
	if err != nil {
		return fmt.Errorf("decoded ca cert content failed: %w", err)
	}
	caKeyContent, err := base64.StdEncoding.DecodeString(caKeyEncodedContent)
	if err != nil {
		return fmt.Errorf("decoded ca key content failed: %w", err)
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

	err = m.store.UpdateAccessTokenCerts(ctx, namespace, token)
	if err != nil {
		m.logger.Errorw("write back access token certs failed", "err", err)
		return
	}

	return nil
}

func NewTokenManager(store metastore.AccessToken, cfg config.Config) *Manager {
	return &Manager{
		store:  store,
		cache:  newTokenCache(),
		cfg:    cfg,
		logger: logger.NewLogger("tokenManager"),
	}
}
