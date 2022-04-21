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

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/basenana/go-flow/flow"
	"github.com/basenana/go-flow/fsm"
	"github.com/coreos/etcd/clientv3"
	"reflect"
)

const (
	etcdFlowMetaKeyPrefix  = "storage.flow.metadata."
	etcdFlowMetaKeyTpl     = etcdFlowMetaKeyPrefix + "%s"
	etcdFlowValueKeyPrefix = "storage.flow.value."
	etcdFlowValueKeyTpl    = etcdFlowValueKeyPrefix + "%s"
	etcdTaskKeyTpl         = "storage.flow.%s.task.%s"
)

type EtcdStorage struct {
	Client *clientv3.Client
}

func (e EtcdStorage) GetFlowMeta(flowId flow.FID) (*FlowMeta, error) {
	var metadata *FlowMeta
	getMetaResp, err := e.Client.Get(context.TODO(), fmt.Sprintf(etcdFlowMetaKeyTpl, flowId))
	if err != nil {
		return nil, err
	} else if len(getMetaResp.Kvs) == 0 || len(getMetaResp.Kvs) > 1 {
		return nil, fmt.Errorf("get no or more many flow metadata %s", flowId)
	}
	metadataByte := getMetaResp.Kvs[0].Value
	err = json.Unmarshal(metadataByte, &metadata)
	if err != nil {
		return nil, err
	}
	return metadata, nil
}

func (e EtcdStorage) GetFlow(flowId flow.FID) (flow.Flow, error) {
	metadata, err := e.GetFlowMeta(flowId)
	if err != nil {
		return nil, err
	}

	var valueByte []byte

	if getValueResp, err := e.Client.Get(context.TODO(), fmt.Sprintf(etcdFlowValueKeyTpl, flowId)); err != nil {
		return nil, err
	} else if len(getValueResp.Kvs) == 0 || len(getValueResp.Kvs) > 1 {
		return nil, fmt.Errorf("get no or more many flow value %s", flowId)
	} else {
		valueByte = getValueResp.Kvs[0].Value
	}

	tt := flow.FlowTypes[metadata.Type]
	if tt == nil || tt.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("flow type must be ptr")
	}
	f := reflect.New(tt).Interface()
	err = json.Unmarshal(valueByte, f)
	if err != nil {
		return nil, fmt.Errorf("can't unmarshal value into struct %s, value: %v, err: %v", metadata.Type, string(valueByte), err)
	}
	return f.(flow.Flow), nil
}

func (e EtcdStorage) GetFlows() ([]flow.Flow, error) {
	getMetaResp, err := e.Client.Get(context.TODO(), etcdFlowMetaKeyPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	metas := make([]FlowMeta, len(getMetaResp.Kvs))
	metaMap := make(map[flow.FID]FlowMeta)
	for i, meta := range getMetaResp.Kvs {
		err := json.Unmarshal(meta.Value, &metas[i])
		if err != nil {
			return nil, err
		}
		metaMap[metas[i].Id] = metas[i]
	}

	getValueResp, err := e.Client.Get(context.TODO(), etcdFlowValueKeyPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	values := make([]flow.Flow, len(getValueResp.Kvs))
	for i, v := range getValueResp.Kvs {
		tt := flow.FlowTypes[metaMap[flow.FID(v.Key)].Type]
		if tt == nil || tt.Kind() != reflect.Ptr {
			return nil, fmt.Errorf("flow type must be ptr")
		}
		f := reflect.New(tt).Interface()
		err := json.Unmarshal(v.Value, f)
		if err != nil {
			return nil, err
		}
		values[i] = f.(flow.Flow)
	}
	return values, nil
}

func (e EtcdStorage) SaveFlow(flow flow.Flow) error {
	metadata := &FlowMeta{
		Type:   flow.Type(),
		Id:     flow.ID(),
		Status: flow.GetStatus(),
	}

	metaByte, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	valueByte, err := json.Marshal(flow)
	if err != nil {
		return err
	}

	_, err = e.Client.KV.Txn(context.TODO()).Then(
		clientv3.OpPut(fmt.Sprintf(etcdFlowMetaKeyTpl, flow.ID()), string(metaByte)),
		clientv3.OpPut(fmt.Sprintf(etcdFlowValueKeyTpl, flow.ID()), string(valueByte)),
	).Commit()
	return err
}

func (e EtcdStorage) DeleteFlow(flowId flow.FID) error {
	_, err := e.Client.KV.Txn(context.TODO()).Then(
		clientv3.OpDelete(fmt.Sprintf(etcdFlowValueKeyTpl, flowId)),
		clientv3.OpDelete(fmt.Sprintf(etcdFlowMetaKeyTpl, flowId)),
	).Commit()
	return err
}

func (e EtcdStorage) SaveTask(flowId flow.FID, task flow.Task) error {
	metadata, err := e.GetFlowMeta(flowId)
	if err != nil {
		return err
	}

	taskStatus := task.GetStatus()
	if metadata.TaskStatus == nil {
		metadata.TaskStatus = map[flow.TName]fsm.Status{}
	}
	metadata.TaskStatus[task.Name()] = taskStatus

	metaByte, err := json.Marshal(metadata)
	if err != nil {
		return err
	}

	taskByte, err := json.Marshal(task)
	if err != nil {
		return err
	}
	_, err = e.Client.KV.Txn(context.TODO()).Then(
		clientv3.OpPut(fmt.Sprintf(etcdFlowMetaKeyTpl, flowId), string(metaByte)),
		clientv3.OpPut(fmt.Sprintf(etcdTaskKeyTpl, flowId, task.Name()), string(taskByte)),
	).Commit()

	return err
}

func (e EtcdStorage) DeleteTask(flowId flow.FID, taskName flow.TName) error {
	if _, err := e.Client.Delete(context.TODO(), fmt.Sprintf(etcdTaskKeyTpl, flowId, taskName)); err != nil {
		return err
	}
	return nil
}

func NewEtcdStorage(client *clientv3.Client) Interface {
	return &EtcdStorage{
		client,
	}
}
