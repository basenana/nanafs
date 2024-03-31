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
	"github.com/basenana/nanafs/cmd/apps/apis/pathmgr"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/pkg/controller"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/storage"
	"github.com/basenana/nanafs/utils/logger"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"testing"
)

var (
	ctrl          controller.Controller
	testServer    *grpc.Server
	serviceClient *Client
	mockListen    *bufconn.Listener
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

	ctrl, err = controller.New(mockConfig{}, memMeta)
	Expect(err).Should(BeNil())

	pm, err := pathmgr.New(ctrl)
	Expect(err).Should(BeNil())

	buffer := 1024 * 1024
	mockListen = bufconn.Listen(buffer)

	testServer = grpc.NewServer()
	_, err = InitServices(testServer, ctrl, pm)
	Expect(err).Should(BeNil())

	go func() {
		if err := testServer.Serve(mockListen); err != nil {
			Expect(err).Should(BeNil())
		}
	}()

	conn, err := grpc.DialContext(
		context.Background(), "bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDisableRetry(),
	)
	Expect(err).Should(BeNil())

	serviceClient = &Client{
		DocumentClient:   NewDocumentClient(conn),
		EntriesClient:    NewEntriesClient(conn),
		InboxClient:      NewInboxClient(conn),
		PropertiesClient: NewPropertiesClient(conn),
		NotifyClient:     NewNotifyClient(conn),
	}
})

var _ = AfterSuite(func() {
	err := mockListen.Close()
	Expect(err).Should(BeNil())
	testServer.Stop()
})

type Client struct {
	DocumentClient
	EntriesClient
	InboxClient
	PropertiesClient
	NotifyClient
}

type mockConfig struct{}

var _ config.Loader = mockConfig{}

func (m mockConfig) GetConfig() (config.Config, error) {
	var cfg = config.Config{
		FS:       &config.FS{Owner: config.FSOwner{Uid: 0, Gid: 0}, Writeback: false},
		Meta:     config.Meta{Type: metastore.MemoryMeta},
		Storages: []config.Storage{{ID: "test-memory-0", Type: storage.MemoryStorage}},
	}
	return cfg, nil
}

func dialer(context.Context, string) (net.Conn, error) {
	return mockListen.Dial()
}
