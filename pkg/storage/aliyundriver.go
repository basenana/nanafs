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
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/basenana/nanafs/config"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
	"github.com/tickstep/aliyunpan-api/aliyunpan"
	"github.com/tickstep/aliyunpan-api/aliyunpan/apierror"
	"go.uber.org/zap"
	"io"
	"os"
	"sync"
	"time"
)

const (
	AliyunDriverStorage           = config.AliyunDriverStorage
	UnofficialAliyunDriverStorage = config.AliyunDriverStorage1
	aliyunDriverReadLimitEnvKey   = "STORAGE_ALIYUN_DRIVER_READ_LIMIT"
	aliyunDriverWriteLimitEnvKey  = "STORAGE_ALIYUN_DRIVER_WRITE_LIMIT"
)

var (
	appConfig = aliyunpan.AppConfig{
		DeviceId: "648529ab-ea5b-4474-98a3-95ffb9b5cdbe",
	}
)

// aliyunDriverWebTokenStorage
// This storage uses a third-party SDK to operate resources on Aliyun Driver,
// which may pose availability risks. It can be used in backup scenarios.
type aliyunDriverWebTokenStorage struct {
	sid         string
	webToken    *aliyunpan.WebLoginToken
	cli         *aliyunpan.PanClient
	userInfo    *aliyunpan.UserInfo
	autoRefresh sync.Once
	cfg         *config.AliyunDriverConfig
	readRate    *utils.ParallelLimiter
	writeRate   *utils.ParallelLimiter
	logger      *zap.SugaredLogger
}

func (u *aliyunDriverWebTokenStorage) ID() string {
	return u.sid
}

func (u *aliyunDriverWebTokenStorage) login(ctx context.Context) error {
	if err := u.refresh(ctx); err != nil {
		return fmt.Errorf("aliyun driver login failed: %s", err)
	}

	userInfo, err := u.cli.GetUserInfo()
	if err != nil {
		return fmt.Errorf("get userinfo failed: %s", err)
	}
	u.logger.Infof("aliyun driver [%s] login succeed", userInfo.UserName)
	u.userInfo = userInfo

	u.autoRefresh.Do(func() {
		go func() {
			var (
				timer       = time.NewTimer(time.Duration(u.webToken.ExpiresIn-60) * time.Second)
				failedTimes = 0
			)

			for {
				select {
				case <-timer.C:
					if err := u.refresh(context.Background()); err != nil {
						failedTimes += 1
						u.logger.Errorf("refresh aliyun driver token error: %s, next retry after %dmin", err, failedTimes)
						timer.Reset(time.Minute * time.Duration(failedTimes))
						continue
					}
					failedTimes = 0
					timer.Reset(time.Duration(u.webToken.ExpiresIn-60) * time.Second)
				}
			}
		}()
	})
	return nil
}

func (u *aliyunDriverWebTokenStorage) refresh(ctx context.Context) error {
	webToken, err := aliyunpan.GetAccessTokenFromRefreshToken(u.cfg.RefreshToken)
	if err != nil {
		return fmt.Errorf("GetAccessTokenFromRefreshToken failed: %s", err)
	}
	u.webToken = webToken
	u.logger.Infow("refresh aliyun driver token succeed", "expiresIn", webToken.ExpiresIn)

	if u.cli == nil {
		u.cli = aliyunpan.NewPanClient(*webToken, aliyunpan.AppLoginToken{}, appConfig,
			aliyunpan.SessionConfig{DeviceName: "550W", ModelName: "NanaFS"})
	} else {
		u.cli.UpdateToken(*webToken)
	}
	_, err = u.cli.CreateSession(nil)
	if err != nil {
		return fmt.Errorf("create session error: %s", err)
	}
	return nil
}

func (u *aliyunDriverWebTokenStorage) Get(ctx context.Context, key, idx int64) (io.ReadCloser, error) {
	path := aliyunDriverObjectPath(key, idx)
	fileInfo, err := u.cli.FileInfoByPath(u.userInfo.FileDriveId, path)
	if err != nil && err.Code != apierror.ApiCodeOk {
		return nil, fmt.Errorf("query file info %s failed: %s", path, err)
	}
	downLoadUrl, err := u.cli.GetFileDownloadUrl(&aliyunpan.GetFileDownloadUrlParam{
		DriveId:   u.userInfo.FileDriveId,
		FileId:    fileInfo.FileId,
		ExpireSec: 60 * 10,
	})
	if err != nil && err.Code != apierror.ApiCodeOk {
		return nil, fmt.Errorf("get file download url %s failed: %s", path, err)
	}

	buf := &aliyunFileBuffer{data: utils.NewMemoryBlock(fileInfo.FileSize)}
	err = u.cli.DownloadFileDataAndSave(downLoadUrl.Url, aliyunpan.FileDownloadRange{}, buf)
	if err != nil && err.Code != apierror.ApiCodeOk {
		return nil, fmt.Errorf("file download %s failed: %s", path, err)
	}
	return buf, nil
}

func (u *aliyunDriverWebTokenStorage) Put(ctx context.Context, key, idx int64, dataReader io.Reader) error {
	dirPath := aliyunDriverObjectDir(key)
	dirInfo, _ := u.cli.FileInfoByPath(u.userInfo.FileDriveId, dirPath)
	if dirInfo == nil {
		// try mkdir
		_, err := u.cli.Mkdir(u.userInfo.FileDriveId, "", dirPath)
		if err != nil && err.Code != apierror.ApiCodeOk {
			u.logger.Errorw("create group dir failed", "dir", dirPath, "err", err)
			return fmt.Errorf("create group dir %s failed: %s", dirPath, err)
		}
		dirInfo, err = u.cli.FileInfoByPath(u.userInfo.FileDriveId, dirPath)
		if err != nil && err.Code != apierror.ApiCodeOk {
			u.logger.Errorw("query group dir info failed", "dir", dirPath, "err", err)
			return fmt.Errorf("query group dir %s failed: %s", dirPath, err)
		}
	}
	var (
		payload         = &aliyunFileBuffer{data: utils.NewMemoryBlock(1 << 21)} // 2M buffer
		payloadErr      error
		payloadSize     int64
		payloadSha1     string
		payloadPath     = aliyunDriverObjectPath(key, idx)
		payloadBasename = aliyunDriverObjectBasename(key, idx)
	)
	// copy content to buffer for hash compute
	hash := sha1.New()
	mWriter := io.MultiWriter(hash, payload)
	payloadSize, payloadErr = io.Copy(mWriter, dataReader)
	if payloadErr != nil {
		u.logger.Errorw("preload payload data failed", "object", payloadPath, "err", payloadErr)
		return payloadErr
	}
	payloadSha1 = hex.EncodeToString(hash.Sum(nil))
	_, _ = payload.Seek(0, io.SeekStart)

	proofCode := aliyunpan.CalcProofCode(u.cli.GetAccessToken(), payload, payloadSize)
	uploadReq, err := u.cli.CreateUploadFile(&aliyunpan.CreateFileUploadParam{
		Name:            payloadBasename,
		Size:            payloadSize,
		ParentFileId:    dirInfo.FileId,
		PartInfoList:    []aliyunpan.FileUploadPartInfoParam{{PartNumber: 1}},
		ProofCode:       proofCode,
		ProofVersion:    "v1",
		ContentHashName: "sha1",
		ContentHash:     payloadSha1,
		CheckNameMode:   "overwrite",
		DriveId:         u.userInfo.FileDriveId,
	})
	if err != nil && err.Code != apierror.ApiCodeOk {
		return fmt.Errorf("create update request failed: %s", err)
	}
	if len(uploadReq.PartInfoList) != 1 {
		return fmt.Errorf("create update request failed: split file part failed, expect=1, got=%d", len(uploadReq.PartInfoList))
	}

	_, _ = payload.Seek(0, io.SeekStart)
	err = u.cli.UploadDataChunk(uploadReq.PartInfoList[0].UploadURL, &aliyunpan.FileUploadChunkData{
		Reader:    payload,
		ChunkSize: payloadSize,
	})
	if err != nil {
		u.logger.Errorw("upload object to aliyun driver chunk failed", "object", payloadPath, "err", err)
		return fmt.Errorf("upload to aliyun driver error: %s", err)
	}

	_, err = u.cli.CompleteUploadFile(
		&aliyunpan.CompleteUploadFileParam{DriveId: u.userInfo.FileDriveId, UploadId: uploadReq.UploadId, FileId: uploadReq.FileId})
	if err != nil && err.Code != apierror.ApiCodeOk {
		return fmt.Errorf("check upload result failed: %s", err)
	}
	u.logger.Infof("upload object %s to aliyun driver finish", payloadPath)
	return nil
}

func (u *aliyunDriverWebTokenStorage) Delete(ctx context.Context, key int64) error {
	dirPath := aliyunDriverObjectDir(key)
	fileInfo, err := u.cli.FileInfoByPath(u.userInfo.FileDriveId, dirPath)
	if err != nil && err.Code != apierror.ApiCodeOk {
		return fmt.Errorf("query key group dir info %s failed: %s", dirPath, err)
	}

	needDelete := []*aliyunpan.FileBatchActionParam{
		{
			DriveId: u.userInfo.FileDriveId,
			FileId:  fileInfo.FileId,
		},
	}
	_, err = u.cli.FileDelete(needDelete)
	if err != nil && err.Code != apierror.ApiCodeOk {
		return fmt.Errorf("batch delete group dir %s failed: %s", dirPath, err)
	}
	_, err = u.cli.RecycleBinFileDelete(needDelete)
	if err != nil && err.Code != apierror.ApiCodeOk {
		return fmt.Errorf("batch cleanup group dir %s failed: %s", dirPath, err)
	}

	//TODO: check this
	//var (
	//	groupList *aliyunpan.FileListResult
	//)
	//for {
	//	groupList, err = u.cli.FileList(&aliyunpan.FileListParam{
	//		DriveId:      u.userInfo.FileDriveId,
	//		ParentFileId: fileInfo.FileId,
	//		Limit:        100,
	//	})
	//	if len(groupList.FileList) == 0 {
	//		break
	//	}
	//
	//	needDelete := make([]*aliyunpan.FileBatchActionParam, len(groupList.FileList))
	//	for i, f := range groupList.FileList {
	//		needDelete[i] = &aliyunpan.FileBatchActionParam{
	//			DriveId: u.userInfo.FileDriveId,
	//			FileId:  f.FileId,
	//		}
	//		u.logger.Infow("batch delete file", "group", dirPath, "file", f.FileName)
	//	}
	//	_, err = u.cli.FileDelete(needDelete)
	//	if err != nil && err.Code != apierror.ApiCodeOk {
	//		return fmt.Errorf("batch delete group dir %s failed: %s", dirPath, err)
	//	}
	//	_, err = u.cli.RecycleBinFileDelete(needDelete)
	//	if err != nil && err.Code != apierror.ApiCodeOk {
	//		return fmt.Errorf("batch cleanup group dir %s failed: %s", dirPath, err)
	//	}
	//}
	return nil
}

func (u *aliyunDriverWebTokenStorage) Head(ctx context.Context, key int64, idx int64) (Info, error) {
	path := aliyunDriverObjectPath(key, idx)
	res, err := u.cli.FileInfoByPath(u.userInfo.FileDriveId, path)
	if err != nil && err.Code != apierror.ApiCodeOk {
		return Info{}, fmt.Errorf("head file %s failed: %s", path, err)
	}
	return Info{Key: path, Size: res.FileSize}, nil
}

func newUnofficialAliyunDriverStorage(storageType, storageID string, cfg *config.AliyunDriverConfig) (Storage, error) {
	if storageType != UnofficialAliyunDriverStorage {
		return nil, fmt.Errorf("aliyun driver not support")
	}

	if cfg.RefreshToken == "" {
		return nil, fmt.Errorf("aliyun driver refresh token is empty")
	}

	s := &aliyunDriverWebTokenStorage{
		sid:       storageID,
		cfg:       cfg,
		readRate:  utils.NewParallelLimiter(str2Int(os.Getenv(aliyunDriverReadLimitEnvKey), 10)),
		writeRate: utils.NewParallelLimiter(str2Int(os.Getenv(aliyunDriverWriteLimitEnvKey), 5)),
		logger:    logger.NewLogger("aliyunDriverWebToken"),
	}
	return s, s.login(context.TODO())
}

func aliyunDriverObjectBasename(key, idx int64) string {
	return fmt.Sprintf("%d_%d", key, idx)
}

func aliyunDriverObjectPath(key, idx int64) string {
	return fmt.Sprintf("/aliyundriver/chunks/%d/%d/%d_%d", key/100, key, key, idx)
}

func aliyunDriverObjectDir(key int64) string {
	return fmt.Sprintf("/aliyundriver/chunks/%d/%d", key/100, key)
}

// aliyunFileBuffer is another implementation of Reader/Writer based on memory.
// It is a utility method for file operations in the aliyun driver storage.
// I've lost count of how many types of Reader/Writer implementations I've written in this project.
type aliyunFileBuffer struct {
	data     []byte
	dataSize int64
	dataOff  int64
}

func (b *aliyunFileBuffer) Read(p []byte) (n int, err error) {
	if b.dataOff >= b.dataSize {
		return 0, io.EOF
	}

	n = copy(p, b.data[b.dataOff:b.dataSize])
	b.dataOff += int64(n)

	return n, nil
}

func (b *aliyunFileBuffer) Write(p []byte) (n int, err error) {
	if int64(len(p))+b.dataOff > int64(len(b.data)) {
		newData := utils.NewMemoryBlock(int64(len(p)) + b.dataOff)
		copy(newData, b.data[:b.dataSize])
		utils.ReleaseMemoryBlock(b.data)
		b.data = newData
	}

	n = copy(b.data[b.dataOff:], p)
	if b.dataOff+int64(n) > b.dataSize {
		b.dataSize = b.dataOff + int64(n)
	}
	return
}

func (b *aliyunFileBuffer) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= b.dataSize {
		return 0, io.EOF
	}
	n = copy(p, b.data[off:b.dataSize])
	return n, nil
}

func (b *aliyunFileBuffer) WriteAt(p []byte, off int64) (n int, err error) {
	if off+int64(len(p)) > int64(len(b.data)) {
		newData := utils.NewMemoryBlock(off + int64(len(p)))
		copy(newData, b.data[:b.dataSize])
		utils.ReleaseMemoryBlock(b.data)
		b.data = newData
	}

	n = copy(b.data[off:], p)
	if int64(n)+off > b.dataSize {
		b.dataSize = int64(n) + off
	}
	return
}

func (b *aliyunFileBuffer) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		b.dataOff = offset
	case io.SeekCurrent:
		b.dataOff += offset
	case io.SeekEnd:
		b.dataOff = b.dataSize + offset
	}
	return b.dataOff, nil
}

func (b *aliyunFileBuffer) Len() int64 {
	return b.dataSize
}

func (b *aliyunFileBuffer) Close() error {
	utils.ReleaseMemoryBlock(b.data)
	b.data = nil
	return nil
}
