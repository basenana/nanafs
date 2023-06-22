/*
   Copyright 2022 Go-Flow Authors

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

package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/coreos/etcd/clientv3"
	"sync"
)

var ErrNotFound = fmt.Errorf("not found")

type Storage interface {
	GetFlow(ctx context.Context, flowId string) (*Flow, error)
	SaveFlow(ctx context.Context, flow *Flow) error
	DeleteFlow(ctx context.Context, flowId string) error
	SaveTask(ctx context.Context, flowId string, task *Task) error
}

const (
	etcdFlowValueKeyPrefix = "storage.flow.value."
	etcdFlowValueKeyTpl    = etcdFlowValueKeyPrefix + "%s"
	etcdTaskKeyTpl         = "storage.flow.%s.task.%s"
)

type EtcdStorage struct {
	Client *clientv3.Client
}

func (e EtcdStorage) GetFlow(ctx context.Context, flowId string) (*Flow, error) {
	var valueByte []byte
	if getValueResp, err := e.Client.Get(ctx, fmt.Sprintf(etcdFlowValueKeyTpl, flowId)); err != nil {
		return nil, err
	} else if len(getValueResp.Kvs) == 0 || len(getValueResp.Kvs) > 1 {
		return nil, fmt.Errorf("get no or more many flow value %s", flowId)
	} else {
		valueByte = getValueResp.Kvs[0].Value
	}

	result := &Flow{}
	err := json.Unmarshal(valueByte, result)
	if err != nil {
		return nil, fmt.Errorf("can't unmarshal flow value, value: %v, err: %v", string(valueByte), err)
	}
	return result, nil
}

func (e EtcdStorage) GetFlows() ([]*Flow, error) {
	getValueResp, err := e.Client.Get(context.TODO(), etcdFlowValueKeyPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	values := make([]*Flow, len(getValueResp.Kvs))
	for i, v := range getValueResp.Kvs {
		f := &Flow{}
		err = json.Unmarshal(v.Value, f)
		if err != nil {
			return nil, err
		}
		values[i] = f
	}
	return values, nil
}

func (e EtcdStorage) SaveFlow(ctx context.Context, flow *Flow) error {
	valueByte, err := json.Marshal(flow)
	if err != nil {
		return err
	}

	_, err = e.Client.KV.Txn(ctx).Then(
		clientv3.OpPut(fmt.Sprintf(etcdFlowValueKeyTpl, flow.ID), string(valueByte)),
	).Commit()
	return err
}

func (e EtcdStorage) DeleteFlow(ctx context.Context, flowId string) error {
	_, err := e.Client.KV.Txn(ctx).Then(
		clientv3.OpDelete(fmt.Sprintf(etcdFlowValueKeyTpl, flowId)),
	).Commit()
	return err
}

func (e EtcdStorage) SaveTask(ctx context.Context, flowId string, task *Task) error {
	taskByte, err := json.Marshal(task)
	if err != nil {
		return err
	}
	_, err = e.Client.KV.Txn(ctx).Then(
		clientv3.OpPut(fmt.Sprintf(etcdTaskKeyTpl, flowId, task.Name), string(taskByte)),
	).Commit()

	return err
}

func NewEtcdStorage(client *clientv3.Client) Storage {
	return &EtcdStorage{client}
}

const (
	flowKeyTpl = "storage.flow.%s"
	taskKeyTpl = "storage.flow.%s.task.%s"
)

type MemStorage struct {
	sync.Map
}

func (m *MemStorage) GetFlow(ctx context.Context, flowId string) (*Flow, error) {
	k := fmt.Sprintf(flowKeyTpl, flowId)
	obj, ok := m.Load(k)
	if !ok {
		return nil, ErrNotFound
	}
	return obj.(*Flow), nil
}

func (m *MemStorage) SaveFlow(ctx context.Context, flow *Flow) error {
	k := fmt.Sprintf(flowKeyTpl, flow.ID)
	m.Store(k, flow)
	return nil
}

func (m *MemStorage) DeleteFlow(ctx context.Context, flowId string) error {
	k := fmt.Sprintf(flowKeyTpl, flowId)
	_, ok := m.LoadAndDelete(k)
	if !ok {
		return ErrNotFound
	}
	return nil
}

func (m *MemStorage) SaveTask(ctx context.Context, flowId string, task *Task) error {
	k := fmt.Sprintf(taskKeyTpl, flowId, task.Name)
	m.Store(k, task)
	return nil
}

func NewInMemoryStorage() Storage {
	return &MemStorage{}
}
