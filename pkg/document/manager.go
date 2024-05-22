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

package document

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/basenana/nanafs/config"

	"go.uber.org/zap"

	"github.com/basenana/nanafs/pkg/dentry"
	"github.com/basenana/nanafs/pkg/events"
	"github.com/basenana/nanafs/pkg/metastore"
	"github.com/basenana/nanafs/pkg/types"
	"github.com/basenana/nanafs/utils"
	"github.com/basenana/nanafs/utils/logger"
)

const (
	attrSourcePluginPrefix = "org.basenana.plugin.source/"

	rssPostMetaID        = "org.basenana.plugin.rss/id"
	rssPostMetaLink      = "org.basenana.plugin.rss/link"
	rssPostMetaTitle     = "org.basenana.plugin.rss/title"
	rssPostMetaUpdatedAt = "org.basenana.plugin.rss/updated_at"
)

type Manager interface {
	ListDocuments(ctx context.Context, filter types.DocFilter) ([]*types.Document, error)
	QueryDocuments(ctx context.Context, query string) ([]*types.Document, error)
	SaveDocument(ctx context.Context, doc *types.Document) error
	GetDocument(ctx context.Context, id int64) (*types.Document, error)
	GetDocumentByEntryId(ctx context.Context, oid int64) (*types.Document, error)
	DeleteDocument(ctx context.Context, id int64) error

	EnableGroupFeed(ctx context.Context, id int64, feedID string) error
	DisableGroupFeed(ctx context.Context, id int64) error
	GetDocsByFeedId(ctx context.Context, feedID string, count int) (*types.FeedResult, error)

	CreateFridayAccount(ctx context.Context, account *types.FridayAccount) error
}

type manager struct {
	recorder metastore.DEntry
	entryMgr dentry.Manager
	indexer  *Indexer
	cfg      config.Loader
	logger   *zap.SugaredLogger
}

var _ Manager = &manager{}

func NewManager(recorder metastore.DEntry, entryMgr dentry.Manager, cfg config.Loader) (Manager, error) {
	docLogger := logger.NewLogger("document")
	docMgr := &manager{
		recorder: recorder,
		entryMgr: entryMgr,
		cfg:      cfg,
		logger:   docLogger,
	}
	err := registerDocExecutor(docMgr)
	if err != nil {
		return nil, err
	}

	indexerCfg, err := cfg.GetSystemConfig(context.Background(), config.DocConfigGroup, "index.local_indexer_dir").String()
	if err == nil {
		docMgr.indexer, err = NewDocumentIndexer(recorder, indexerCfg)
		if err != nil {
			return nil, err
		}
	} else {
		docMgr.logger.Warnw("skip open document indexer", "err", err)
	}

	return docMgr, nil
}

func (m *manager) ListDocuments(ctx context.Context, filter types.DocFilter) ([]*types.Document, error) {
	result, err := m.recorder.ListDocument(ctx, filter)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (m *manager) QueryDocuments(ctx context.Context, query string) ([]*types.Document, error) {
	if m.indexer == nil {
		return nil, fmt.Errorf("indexer not enable")
	}
	return m.indexer.Query(ctx, query, QueryDialectBleve)
}

func (m *manager) SaveDocument(ctx context.Context, doc *types.Document) error {
	var (
		crtDoc *types.Document
		err    error
	)
	crtDoc, err = m.recorder.GetDocument(ctx, doc.ID)
	if err != nil && !errors.Is(err, types.ErrNotFound) {
		return err
	}
	if crtDoc == nil {
		crtDoc, err = m.recorder.GetDocumentByEntryId(ctx, doc.OID)
		if err != nil {
			if errors.Is(err, types.ErrNotFound) {
				// create new one
				doc.ID = utils.GenerateNewID()
				doc.CreatedAt = time.Now()
				doc.ChangedAt = time.Now()
				if err = m.recorder.SaveDocument(ctx, doc); err != nil {
					m.logger.Errorw("create document failed", "document", doc.ID, "err", err)
					return err
				}
				m.publicDocActionEvent(events.ActionTypeCreate, doc)
				return nil
			}
			m.logger.Errorw("create document failed", "err", err)
			return err
		}
	}
	// update
	if doc.OID != 0 && crtDoc.OID != doc.OID {
		return errors.New("can't update oid of doc")
	}
	if doc.Name != "" {
		crtDoc.Name = doc.Name
	}
	if doc.ParentEntryID != 0 {
		crtDoc.ParentEntryID = doc.ParentEntryID
	}
	if doc.Summary != "" {
		crtDoc.Summary = doc.Summary
	}
	if len(doc.KeyWords) != 0 {
		crtDoc.KeyWords = doc.KeyWords
	}
	if doc.Content != "" {
		crtDoc.Content = doc.Content
	}
	if doc.Desync != nil {
		crtDoc.Desync = doc.Desync
	}
	if doc.Marked != nil {
		crtDoc.Marked = doc.Marked
	}
	if doc.Unread != nil {
		crtDoc.Unread = doc.Unread
	}
	crtDoc.ChangedAt = time.Now()
	if err = m.recorder.SaveDocument(ctx, crtDoc); err != nil {
		m.logger.Errorw("update document failed", "document", doc.ID, "err", err)
		return err
	}
	m.publicDocActionEvent(events.ActionTypeUpdate, crtDoc)
	return nil
}

func (m *manager) GetDocument(ctx context.Context, id int64) (*types.Document, error) {
	return m.recorder.GetDocument(ctx, id)
}

func (m *manager) GetDocumentByEntryId(ctx context.Context, oid int64) (*types.Document, error) {
	return m.recorder.GetDocumentByEntryId(ctx, oid)
}

func (m *manager) DeleteDocument(ctx context.Context, id int64) error {
	doc, err := m.GetDocument(ctx, id)
	if err != nil {
		return err
	}
	err = m.recorder.DeleteDocument(ctx, id)
	if err != nil {
		return err
	}
	m.publicDocActionEvent(events.ActionTypeDestroy, doc)
	return nil
}

func (m *manager) GetDocsByFeedId(ctx context.Context, feedID string, count int) (*types.FeedResult, error) {
	feed, err := m.recorder.GetDocumentFeed(ctx, feedID)
	if err != nil {
		m.logger.Errorw("query feed info error", "feed", feedID, "err", err)
		return nil, err
	}

	result := &types.FeedResult{
		FeedId:    feedID,
		GroupName: feed.Display,
		SiteName:  feed.Display,
	}

	var documents []*types.Document
	switch {
	case feed.ParentID != 0:
		parentEn, err := m.recorder.GetEntry(ctx, feed.ParentID)
		if err != nil {
			m.logger.Errorw("query feed parent entry failed", "feed", feedID, "entry", feed.ParentID, "err", err)
			return nil, err
		}
		result.GroupName = parentEn.Name

		gExtend, err := m.recorder.GetEntryExtendData(ctx, parentEn.ID)
		if err != nil {
			m.logger.Errorw("get group extendData failed", "feed", feedID, "entry", parentEn.ID, "err", err)
			return nil, err
		}
		for key, val := range gExtend.Properties.Fields {
			switch key {
			case attrSourcePluginPrefix + "site_url":
				result.SiteUrl = val.Value
			case attrSourcePluginPrefix + "site_name":
				result.SiteName = val.Value
			case attrSourcePluginPrefix + "feed_url":
				result.FeedUrl = val.Value

			}
		}
		documents, err = m.recorder.ListDocument(ctx, types.DocFilter{ParentID: feed.ParentID})
		if err != nil {
			return nil, err
		}
	case feed.Keywords != "":
		parts := strings.Split(feed.Keywords, ",")
		phrases := make([]string, len(parts))
		for i := range parts {
			phrases[i] = fmt.Sprintf("\"%s\"", parts[i])
		}
		feed.IndexQuery = strings.Join(phrases, " ")
		fallthrough
	case feed.IndexQuery != "":
		documents, err = m.QueryDocuments(ctx, feed.IndexQuery)
		if err != nil {
			return nil, err
		}
		sort.Slice(documents, func(i, j int) bool {
			return documents[i].ChangedAt.Before(documents[j].CreatedAt)
		})
	}

	if len(documents) > count {
		documents = documents[:count]
	}

	docFeeds := make([]types.FeedResultItem, len(documents))
	for i, doc := range documents {
		eExtend, err := m.recorder.GetEntryExtendData(ctx, doc.OID)
		if err != nil {
			m.logger.Errorw("get entry extendData failed", "feed", feedID, "entry", doc.OID, "err", err)
			return nil, err
		}
		updatedAt := eExtend.Properties.Fields[rssPostMetaUpdatedAt].Value
		if updatedAt == "" {
			updatedAt = doc.CreatedAt.Format(time.RFC3339)
		}
		docFeeds[i] = types.FeedResultItem{
			ID:        eExtend.Properties.Fields[rssPostMetaID].Value,
			Title:     eExtend.Properties.Fields[rssPostMetaTitle].Value,
			Link:      eExtend.Properties.Fields[rssPostMetaLink].Value,
			UpdatedAt: updatedAt,
			Document:  *doc,
		}
	}
	result.Documents = docFeeds

	return result, nil
}

func (m *manager) EnableGroupFeed(ctx context.Context, id int64, feedID string) error {
	entry, err := m.recorder.GetEntry(ctx, id)
	if err != nil {
		m.logger.Errorw("check feed group error", "feed", feedID, "entry", id, "err", err)
		return err
	}

	if !entry.IsGroup {
		return types.ErrNoGroup
	}

	existedFeed, err := m.recorder.GetDocumentFeed(ctx, feedID)
	if err != nil && err != types.ErrNotFound {
		m.logger.Errorw("check feed existed error", "feed", feedID, "err", err)
		return err
	}

	if existedFeed != nil && existedFeed.ParentID == id {
		return nil
	}

	feed := types.DocumentFeed{ID: feedID, Display: entry.Name, ParentID: id}
	err = m.recorder.EnableDocumentFeed(ctx, feed)
	if err != nil {
		m.logger.Errorw("enable group feed failed", "entry", id, "feed", feedID, "err", err)
		return err
	}
	return nil
}

func (m *manager) DisableGroupFeed(ctx context.Context, id int64) error {
	err := m.recorder.DisableDocumentFeed(ctx, types.DocumentFeed{ParentID: id})
	if err != nil {
		m.logger.Errorw("disable group feed failed", "entry", id, "err", err)
		return err
	}
	return nil
}

func (m *manager) CreateFridayAccount(ctx context.Context, account *types.FridayAccount) error {
	account.CreatedAt = time.Now()
	account.ID = utils.GenerateNewID()
	err := m.recorder.CreateFridayAccount(ctx, account)
	if err != nil {
		m.logger.Errorw("save friday account failed", "refId", account.RefID, "refType", account.RefType, "err", err)
		return err
	}
	return nil
}

func (m *manager) handleEntryEvent(evt *types.Event) error {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Hour)
	defer cancel()

	nsCtx := types.WithNamespace(ctx, types.NewNamespace(evt.Namespace))
	entry := evt.Data

	switch evt.Type {
	case events.ActionTypeDestroy:
		doc, err := m.GetDocumentByEntryId(nsCtx, entry.ID)
		if err != nil {
			if err == types.ErrNotFound {
				return nil
			}
			m.logger.Errorw("[docCleanExecutor] get doc failed", "entry", entry.ID, "err", err)
			return err
		}

		err = m.DeleteDocument(nsCtx, doc.ID)
		if err != nil {
			m.logger.Errorw("[docCleanExecutor] delete doc failed", "entry", entry.ID, "document", doc.ID, "err", err)
			return err
		}
		return nil
	case events.ActionTypeChangeParent:
		fallthrough
	case events.ActionTypeUpdate:
		en, err := m.entryMgr.GetEntry(nsCtx, entry.ID)
		if err != nil {
			m.logger.Errorw("[docUpdateExecutor] get entry failed", "entry", entry.ID, "err", err)
			return err
		}

		doc, err := m.GetDocumentByEntryId(nsCtx, en.ID)
		if err != nil {
			if err == types.ErrNotFound {
				return nil
			}
			m.logger.Errorw("[docUpdateExecutor] get doc failed", "entry", entry.ID, "err", err)
			return err
		}

		if doc.Name != en.Name || doc.ParentEntryID != en.ParentID {
			doc.Name = en.Name
			doc.ParentEntryID = en.ParentID
			err = m.SaveDocument(nsCtx, doc)
			if err != nil {
				m.logger.Errorw("[docUpdateExecutor] update doc failed", "entry", entry.ID, "document", doc.ID, "err", err)
				return err
			}
		}
		return nil
	case events.ActionTypeCompact:
		doc, err := m.GetDocumentByEntryId(nsCtx, entry.ID)
		if err != nil {
			if err == types.ErrNotFound {
				return nil
			}
			m.logger.Errorw("[docDesyncExecutor] get entry failed", "entry", entry.ID, "err", err)
			return err
		}
		if *doc.Desync {
			return nil
		}
		dsync := true
		doc.Desync = &dsync
		err = m.SaveDocument(nsCtx, doc)
		if err != nil {
			m.logger.Errorw("[docDesyncExecutor] update doc failed", "entry", entry.ID, "document", doc.ID, "err", err)
			return err
		}
		return nil
	}

	return nil
}

func registerDocExecutor(docMgr *manager) error {
	eventMappings := []struct {
		topic   string
		action  string
		handler func(*types.Event) error
	}{
		{events.TopicNamespaceEntry, events.ActionTypeDestroy, docMgr.handleEntryEvent},
		{events.TopicNamespaceEntry, events.ActionTypeUpdate, docMgr.handleEntryEvent},
		{events.TopicNamespaceEntry, events.ActionTypeChangeParent, docMgr.handleEntryEvent},
		{events.TopicNamespaceFile, events.ActionTypeCompact, docMgr.handleEntryEvent},
		{events.TopicNamespaceDocument, events.ActionTypeCreate, docMgr.handleDocumentEvent},
		{events.TopicNamespaceDocument, events.ActionTypeUpdate, docMgr.handleDocumentEvent},
		{events.TopicNamespaceDocument, events.ActionTypeDestroy, docMgr.handleDocumentEvent},
	}

	for _, mapping := range eventMappings {
		if _, err := events.Subscribe(events.NamespacedTopic(mapping.topic, mapping.action), mapping.handler); err != nil {
			return fmt.Errorf("register doc event executor failed: %w", err)
		}
	}

	return nil
}
