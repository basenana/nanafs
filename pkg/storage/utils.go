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

package storage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"github.com/pierrec/lz4/v4"
	"io"
	"runtime/trace"
)

type nodeUsingInfo struct {
	filename string
	node     *cacheNode
	index    int
	updateAt int64
}

type priorityNodeQueue []*nodeUsingInfo

func (pq *priorityNodeQueue) Len() int {
	return len(*pq)
}

func (pq *priorityNodeQueue) Less(i, j int) bool {
	if (*pq)[i].node.freq() == (*pq)[j].node.freq() {
		return (*pq)[i].updateAt < (*pq)[j].updateAt
	}
	return (*pq)[i].node.freq() < (*pq)[j].node.freq()
}

func (pq *priorityNodeQueue) Swap(i, j int) {
	(*pq)[i], (*pq)[j] = (*pq)[j], (*pq)[i]
	(*pq)[i].index = i
	(*pq)[j].index = j
}

func (pq *priorityNodeQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*nodeUsingInfo)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityNodeQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

func compress(ctx context.Context, in io.Reader, out io.Writer) error {
	defer trace.StartRegion(ctx, "storage.localCache.compress").End()
	zw := lz4.NewWriter(out)
	defer zw.Close()
	if _, err := io.Copy(zw, in); err != nil {
		return err
	}
	return nil
}

func decompress(ctx context.Context, in io.Reader, out io.Writer) error {
	defer trace.StartRegion(ctx, "storage.localCache.decompress").End()
	zr := lz4.NewReader(in)
	if _, err := io.Copy(out, zr); err != nil {
		if err == io.EOF {
			// https://github.com/golang/go/issues/44411
			return nil
		}
		return err
	}

	return nil
}

func encrypt(ctx context.Context, segIDKey int64, method, secretKey string, in io.Reader, out io.Writer) error {
	defer trace.StartRegion(ctx, "storage.localCache.encrypt").End()
	aesCipher, err := aes.NewCipher([]byte(secretKey))
	if err != nil {
		return err
	}
	stream := cipher.NewCTR(aesCipher, aesCipherIV(segIDKey, aesCipher.BlockSize()))
	return cipherXOR(ctx, stream, in, out)
}

func decrypt(ctx context.Context, segIDKey int64, method, secretKey string, in io.Reader, out io.Writer) error {
	defer trace.StartRegion(ctx, "storage.localCache.decrypt").End()
	aesCipher, err := aes.NewCipher([]byte(secretKey))
	if err != nil {
		return err
	}
	stream := cipher.NewCTR(aesCipher, aesCipherIV(segIDKey, aesCipher.BlockSize()))
	return cipherXOR(ctx, stream, in, out)
}

func aesCipherIV(segIDKey int64, blkSize int) []byte {
	idKeyBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(idKeyBuf, uint64(segIDKey^0x0092094024))
	idKeyHash := sha256.Sum256(idKeyBuf)

	iv := make([]byte, blkSize)
	n := copy(iv, idKeyHash[:sha256.Size])
	if n < blkSize {
		for i := n; i < blkSize; i++ {
			iv[i] = 0
		}
	}
	return iv
}

func cipherXOR(ctx context.Context, s cipher.Stream, in io.Reader, out io.Writer) error {
	var (
		inBuf  = make([]byte, 512)
		outBuf = make([]byte, 512)
	)

	for {
		rN, err := in.Read(inBuf)
		if rN > 0 {
			s.XORKeyStream(outBuf[:rN], inBuf[:rN])
			if _, err = out.Write(outBuf[:rN]); err != nil {
				return err
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func reverseString(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}
