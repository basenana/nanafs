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

package common

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils"
	"log"
)

func ReadRootCAs(cfg config.Loader) (*x509.CertPool, error) {
	var (
		ctx      = context.TODO()
		certPool = x509.NewCertPool()
	)
	clientCAEncoded, err := cfg.GetSystemConfig(ctx, config.AuthConfigGroup, "ca_cert_0").String()
	if err != nil {
		return nil, fmt.Errorf("read ca config error, cert: %w", err)
	}

	ca, err := base64.StdEncoding.DecodeString(clientCAEncoded)
	if err != nil {
		return nil, fmt.Errorf("decode ca cert error: %s", err)
	}
	if ok := certPool.AppendCertsFromPEM(ca); !ok {
		log.Fatal("failed to append ca certs")
	}
	return certPool, nil
}

func ReadClientCAs(cfg config.Loader) (*x509.CertPool, error) {
	return ReadRootCAs(cfg)
}

func EnsureServerX509KeyPair(cfg config.Loader) (*tls.Certificate, error) {
	encodedServerCert, err1 := cfg.GetSystemConfig(context.TODO(), config.FsAPIConfigGroup, "server_cert").String()
	encodedServerKey, err2 := cfg.GetSystemConfig(context.TODO(), config.FsAPIConfigGroup, "server_key").String()
	if err1 != nil && !errors.Is(err1, config.ErrNotConfigured) {
		return nil, fmt.Errorf("load server cert error: %w", err1)
	}
	if err2 != nil && !errors.Is(err2, config.ErrNotConfigured) {
		return nil, fmt.Errorf("load server key error: %w", err2)
	}

	if encodedServerCert == "" || encodedServerKey == "" {
		// init server certs
		return initServerX509KeyPair(cfg)
	}

	serverCert, err := base64.StdEncoding.DecodeString(encodedServerCert)
	serverKey, err := base64.StdEncoding.DecodeString(encodedServerKey)
	certPair, err := tls.X509KeyPair(serverCert, serverKey)
	if err != nil {
		return nil, fmt.Errorf("load cert/key pair error: %w", err)
	}
	return &certPair, nil
}

func initServerX509KeyPair(cfg config.Loader) (*tls.Certificate, error) {
	var ctx = context.Background()
	rootCAEncoded, err := cfg.GetSystemConfig(ctx, config.AuthConfigGroup, "ca_cert_0").String()
	if err != nil {
		return nil, fmt.Errorf("read ca cert error, cert: %w", err)
	}
	rootCAKeyEncoded, err := cfg.GetSystemConfig(ctx, config.AuthConfigGroup, "ca_key_0").String()
	if err != nil {
		return nil, fmt.Errorf("read ca key error, cert: %w", err)
	}

	ct := &utils.CertTool{}
	ct.CaCertPEM, err = base64.StdEncoding.DecodeString(rootCAEncoded)
	if err != nil {
		return nil, err
	}
	ct.CaKeyPEM, err = base64.StdEncoding.DecodeString(rootCAKeyEncoded)
	if err != nil {
		return nil, err
	}

	sn, err := ServiceName(cfg)
	if err != nil {
		return nil, err
	}

	c, k, err := ct.GenerateCertPair("basenana", "nanafs", sn)
	if err != nil {
		return nil, err
	}

	err = cfg.SetSystemConfig(ctx, config.FsAPIConfigGroup, "server_cert", base64.StdEncoding.EncodeToString(c))
	if err != nil {
		return nil, err
	}
	err = cfg.SetSystemConfig(ctx, config.FsAPIConfigGroup, "server_key", base64.StdEncoding.EncodeToString(k))
	if err != nil {
		return nil, err
	}

	certPair, err := tls.X509KeyPair(c, k)
	if err != nil {
		return nil, fmt.Errorf("init cert/key pair error: %w", err)
	}
	return &certPair, nil
}

func ServiceName(cfg config.Loader) (string, error) {
	serviceName, err := cfg.GetSystemConfig(context.TODO(), config.FsAPIConfigGroup, "service_name").String()
	if err != nil && !errors.Is(err, config.ErrNotConfigured) {
		return "", fmt.Errorf("load service name error: %w", err)
	}
	if serviceName == "" {
		serviceName = "nanafs"
	}
	return serviceName, nil
}
