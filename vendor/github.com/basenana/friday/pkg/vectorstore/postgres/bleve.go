package postgres

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/blevesearch/bleve/v2/registry"
	"github.com/blevesearch/upsidedown_store_api"
	"gorm.io/gorm"

	"github.com/basenana/friday/pkg/utils/logger"
)

const (
	PgKVStoreName = "pg_table"
)

func init() {
	registry.RegisterKVStore(PgKVStoreName, pgKVStoreConstructor)
}

func pgKVStoreConstructor(mo store.MergeOperator, config map[string]interface{}) (store.KVStore, error) {
	dsnStr, ok := config["dsn"]
	if !ok {
		return nil, fmt.Errorf("dsn not found")
	}
	pgCli, err := NewPostgresClient(logger.NewLogger("bleve"), dsnStr.(string))
	if err != nil {
		return nil, err
	}
	return &adaptor{PostgresClient: pgCli, mo: mo}, nil
}

type adaptor struct {
	*PostgresClient
	mo store.MergeOperator
}

var _ store.KVStore = &adaptor{}

func (p *adaptor) Writer() (store.KVWriter, error) {
	return NewPgKVWriter(p.PostgresClient, p.mo), nil
}

func (p *adaptor) Reader() (store.KVReader, error) {
	return NewPgKVReader(p.PostgresClient), nil
}

func (p *adaptor) Close() error {
	return nil
}

type PgKVWriter struct {
	*PostgresClient
	mo store.MergeOperator
}

var _ store.KVWriter = &PgKVWriter{}

func NewPgKVWriter(cli *PostgresClient, mo store.MergeOperator) *PgKVWriter {
	return &PgKVWriter{PostgresClient: cli, mo: mo}
}

func (p *PgKVWriter) NewBatch() store.KVBatch {
	return store.NewEmulatedBatch(p.mo)
}

func (p *PgKVWriter) NewBatchEx(options store.KVBatchOptions) ([]byte, store.KVBatch, error) {
	return make([]byte, options.TotalBytes), p.NewBatch(), nil
}

func (p *PgKVWriter) ExecuteBatch(batch store.KVBatch) error {
	emulatedBatch, ok := batch.(*store.EmulatedBatch)
	if !ok {
		return fmt.Errorf("wrong type of batch")
	}

	err := p.dEntity.DB.WithContext(context.Background()).Transaction(func(tx *gorm.DB) error {
		for k, mergeOps := range emulatedBatch.Merger.Merges {
			mod := &BleveKV{ID: bytes2Str([]byte(k)), Key: []byte(k)}

			res := tx.First(mod)
			if res.Error != nil && !errors.Is(res.Error, gorm.ErrRecordNotFound) {
				return res.Error
			}
			mergedVal, fullMergeOk := p.mo.FullMerge(mod.Key, mod.Value, mergeOps)
			if !fullMergeOk {
				return fmt.Errorf("merge operator returned failure")
			}
			mod.Value = mergedVal
			if res.Error == nil {
				res = tx.Updates(mod)
			} else if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				res = tx.Create(mod)
			}
			if res.Error != nil {
				return res.Error
			}
		}

		for _, op := range emulatedBatch.Ops {
			mod := &BleveKV{ID: bytes2Str(op.K), Key: op.K, Value: op.V}
			var res *gorm.DB
			if op.V != nil {
				res = tx.Save(mod)
			} else {
				res = tx.Delete(mod)
			}
			if res.Error != nil {
				return res.Error
			}
		}
		return nil
	})
	return err
}

func (p *PgKVWriter) Close() error {
	p.PostgresClient = nil
	return nil
}

type PgKVReader struct {
	*PostgresClient
}

var _ store.KVReader = &PgKVReader{}

func (p *PgKVReader) Get(key []byte) ([]byte, error) {
	mod := &BleveKV{ID: bytes2Str(key)}
	res := p.dEntity.DB.WithContext(context.Background()).First(mod)
	if res.Error != nil {
		if errors.Is(res.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, res.Error
	}
	return mod.Value, nil
}

func (p *PgKVReader) MultiGet(keys [][]byte) ([][]byte, error) {
	idList := make([]string, len(keys))
	for i, k := range keys {
		idList[i] = bytes2Str(k)
	}

	var modList []BleveKV
	res := p.dEntity.DB.WithContext(context.Background()).Where("id IN ?", idList).Find(modList)
	if res.Error != nil {
		return nil, res.Error
	}
	result := make([][]byte, len(modList))
	for i, mod := range modList {
		result[i] = mod.Value
	}
	return result, nil
}

func (p *PgKVReader) PrefixIterator(prefix []byte) store.KVIterator {
	var modList []BleveKV
	res := p.PostgresClient.dEntity.WithContext(context.Background()).
		Where("id LIKE ?", bytes2Str(prefix)+"%").
		Order("key DESC").Find(&modList)
	if res.Error != nil {
		return &KVIterator{err: res.Error}
	}

	it := &KVIterator{prefix: prefix}
	for i, mod := range modList {
		if bytes.HasPrefix(mod.Key, prefix) {
			it.listKV = append(it.listKV, modList[i])
		}
	}
	return it
}

func (p *PgKVReader) RangeIterator(start, end []byte) store.KVIterator {
	var modList []BleveKV
	res := p.PostgresClient.dEntity.WithContext(context.Background()).
		Where("id >= ? AND id < ?", bytes2Str(start), bytes2Str(end)).
		Order("key DESC").Find(&modList)
	if res.Error != nil {
		return &KVIterator{err: res.Error}
	}

	it := &KVIterator{start: start, end: end}
	for i, mod := range modList {
		if bytes.Compare(mod.Key, start) >= 0 && bytes.Compare(mod.Key, end) < 0 {
			it.listKV = append(it.listKV, modList[i])
		}
	}
	return it
}

func (p *PgKVReader) Close() error {
	return nil
}

func NewPgKVReader(cli *PostgresClient) *PgKVReader {
	return &PgKVReader{PostgresClient: cli}
}

type KVIterator struct {
	listKV     []BleveKV
	start, end []byte
	prefix     []byte
	crt        int
	err        error
}

var _ store.KVIterator = &KVIterator{}

func (i *KVIterator) Seek(key []byte) {
	if i.start != nil && bytes.Compare(key, i.start) < 0 {
		key = i.start
	}
	if i.prefix != nil && !bytes.HasPrefix(key, i.prefix) {
		if bytes.Compare(key, i.prefix) < 0 {
			key = i.prefix
		} else {
			i.crt = len(i.listKV)
			return
		}
	}

	for idx, kv := range i.listKV {
		if bytes.Equal(key, kv.Key) {
			i.crt = idx
		}
	}
}

func (i *KVIterator) Next() {
	i.crt += 1
}

func (i *KVIterator) Key() []byte {
	if i.crt < len(i.listKV) {
		return i.listKV[i.crt].Key
	}
	return nil
}

func (i *KVIterator) Value() []byte {
	if i.crt < len(i.listKV) {
		return i.listKV[i.crt].Value
	}
	return nil
}

func (i *KVIterator) Valid() bool {
	return i.crt < len(i.listKV)
}

func (i *KVIterator) Current() ([]byte, []byte, bool) {
	if i.crt < len(i.listKV) {
		return i.listKV[i.crt].Key, i.listKV[i.crt].Value, true
	}
	return nil, nil, false
}

func (i *KVIterator) Close() error {
	return nil
}

func bytes2Str(b []byte) string {
	return fmt.Sprintf("%X", b)
}
