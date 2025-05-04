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

package v1

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"google.golang.org/grpc/credentials"
	"net"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"

	"github.com/basenana/nanafs/cmd/apps/apis/fsapi/common"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/friday"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils/logger"
)

var (
	dep           *common.Depends
	testMeta      metastore.Meta
	testFriday    friday.Friday
	testServer    *grpc.Server
	serviceClient *Client
	mockListen    *bufconn.Listener

	mockConfig = config.Bootstrap{
		FS:       &config.FS{Owner: config.FSOwner{Uid: 0, Gid: 0}, Writeback: false},
		Meta:     config.Meta{Type: metastore.MemoryMeta},
		Storages: []config.Storage{{ID: "test-memory-0", Type: storage.MemoryStorage}},
	}
)

func TestV1API(t *testing.T) {
	logger.InitLogger()
	defer logger.Sync()
	RegisterFailHandler(Fail)
	RunSpecs(t, "FsAPI V1 Suite")
}

var _ = BeforeSuite(func() {
	memMeta, err := metastore.NewMetaStorage(metastore.MemoryMeta, config.Meta{})
	Expect(err).Should(BeNil())
	testMeta = memMeta

	testFriday = friday.NewMockFriday()

	workdir, err := os.MkdirTemp(os.TempDir(), "ut-nanafs-fsapi-")
	Expect(err).Should(BeNil())

	storage.InitLocalCache(config.Bootstrap{CacheDir: workdir, CacheSize: 0})

	cl := config.NewMockConfigLoader(mockConfig)
	_ = cl.SetSystemConfig(context.TODO(), config.WorkflowConfigGroup, "job_workdir", workdir)

	dep, err = common.InitDepends(cl, memMeta)
	Expect(err).Should(BeNil())

	buffer := 1024 * 1024
	mockListen = bufconn.Listen(buffer)

	// TODO: use token mgr
	serverCreds, clientCreds, err := setupCerts(cl)
	Expect(err).Should(BeNil())
	var opts = []grpc.ServerOption{
		grpc.Creds(serverCreds),
		grpc.MaxRecvMsgSize(1024 * 1024 * 50), // 50M
		common.WithCommonInterceptors(),
		common.WithStreamInterceptors(),
	}
	testServer = grpc.NewServer(opts...)
	_, err = InitServicesV1(testServer, dep)
	Expect(err).Should(BeNil())

	go func() {
		if err := testServer.Serve(mockListen); err != nil {
			Expect(err).Should(BeNil())
		}
	}()

	conn, err := grpc.DialContext(
		context.Background(), "bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(clientCreds),
		grpc.WithDisableRetry(),
	)
	Expect(err).Should(BeNil())

	serviceClient = &Client{
		DocumentClient:   NewDocumentClient(conn),
		RoomClient:       NewRoomClient(conn),
		EntriesClient:    NewEntriesClient(conn),
		PropertiesClient: NewPropertiesClient(conn),
		WorkflowClient:   NewWorkflowClient(conn),
		NotifyClient:     NewNotifyClient(conn),
	}
})

var _ = AfterSuite(func() {
	if mockListen != nil {
		err := mockListen.Close()
		Expect(err).Should(BeNil())
	}
	if testServer != nil {
		testServer.Stop()
	}
})

type Client struct {
	DocumentClient
	RoomClient
	EntriesClient
	PropertiesClient
	WorkflowClient
	NotifyClient
}

func dialer(context.Context, string) (net.Conn, error) {
	return mockListen.Dial()
}

func setupCerts(cfg config.Config) (credentials.TransportCredentials, credentials.TransportCredentials, error) {
	ct := &utils.CertTool{}
	caCert, _, err := ct.GenerateCAPair()
	if err != nil {
		return nil, nil, fmt.Errorf("gen root cert/key file error: %w", err)
	}

	rootCaPool := x509.NewCertPool()
	rootCaPool.AppendCertsFromPEM(caCert)
	clientCaPool := rootCaPool

	serverCert, serverKey, err := ct.GenerateCertPair("basenana", "nanafs", "localhost", "localhost")
	if err != nil {
		return nil, nil, fmt.Errorf("gen server cert/key file error: %w", err)
	}
	certificate, err := tls.X509KeyPair(serverCert, serverKey)
	if err != nil {
		return nil, nil, fmt.Errorf("load server cert/key file error: %w", err)
	}

	serverCreds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{certificate},
		ServerName:   "localhost",
		RootCAs:      rootCaPool,
		ClientCAs:    clientCaPool,
		ClientAuth:   tls.VerifyClientCertIfGiven,
	})

	clientCert, clientKey, err := ct.GenerateCertPair(
		types.DefaultNamespace,     // O
		"ak-test",                  // OU
		fmt.Sprintf("%d,%d", 0, 0), // CN
	)
	if err != nil {
		return nil, nil, fmt.Errorf("gen client cert/key file error: %w", err)
	}
	clientCertificate, err := tls.X509KeyPair(clientCert, clientKey)
	if err != nil {
		return nil, nil, fmt.Errorf("load client cert/key file error: %w", err)
	}

	clientCreds := credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{clientCertificate},
		ServerName:   "localhost",
		RootCAs:      rootCaPool,
	})
	return serverCreds, clientCreds, nil
}
