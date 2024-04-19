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

package utils

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

type CertTool struct {
	CaCertPEM, CaKeyPEM []byte
}

func (c *CertTool) GenerateCAPair() (certContent []byte, keyContent []byte, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return
	}

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Basenana"},
			CommonName:   "basenana.org",
			Country:      []string{"CN"},
			Province:     []string{"ZJ"},
			Locality:     []string{"HZ"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}

	certBytes, _ := x509.CreateCertificate(rand.Reader, cert, cert, &privateKey.PublicKey, privateKey)

	certFile := &bytes.Buffer{}
	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	if err != nil {
		return
	}

	keyFile := &bytes.Buffer{}
	pkContent, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return
	}
	err = pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: pkContent})
	if err != nil {
		return
	}

	c.CaCertPEM = certFile.Bytes()
	c.CaKeyPEM = keyFile.Bytes()
	return c.CaCertPEM, c.CaKeyPEM, nil
}

func (c *CertTool) GenerateCertPair(o, ou, cn string) (certContent []byte, keyContent []byte, err error) {
	caCertBlock, _ := pem.Decode(c.CaCertPEM)
	caCert, err := x509.ParseCertificate(caCertBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse ca cert failed: %w", err)
	}
	caKeyBlock, _ := pem.Decode(c.CaKeyPEM)
	caKey, err := x509.ParsePKCS8PrivateKey(caKeyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse ca key failed: %w", err)
	}

	// new cert pair
	privateKey, _ := rsa.GenerateKey(rand.Reader, 4096)

	cert := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization:       []string{o},
			OrganizationalUnit: []string{ou},
			CommonName:         cn,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(1, 0, 0),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, cert, caCert, &privateKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, err
	}

	certFile := &bytes.Buffer{}
	err = pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	if err != nil {
		return nil, nil, err
	}
	keyFile := &bytes.Buffer{}
	err = pem.Encode(keyFile, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	if err != nil {
		return nil, nil, err
	}
	return certFile.Bytes(), keyFile.Bytes(), nil
}
