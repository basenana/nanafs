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

package types

import (
	"path/filepath"
	"strings"
)

// Kind of Entry
type Kind string

const (
	/*
		system-wide kind
	*/
	GroupKind         = "group"
	SmartGroupKind    = "smtgroup"
	ExternalGroupKind = "extgroup"

	/*
		text based file kind
	*/
	TextKind = "text"

	/*
		format doc kind
	*/
	FmtDocKind = "fmtdoc"
	PdfDocKind = "pdf"

	WordDocKind  = "worddoc"
	ExcelDocKind = "exceldoc"
	PptDocKind   = "pptdoc"

	/*
		media file kind
	*/
	ImageKind = "image"
	VideoKind = "video"
	AudioKind = "audio"

	/*
		web based file kind
	*/
	WebArchiveKind  = "webarchive"
	WebBookmarkKind = "webbbookmark"
	HtmlKind        = "html"

	/*
		ungrouped files
	*/
	RawKind = "raw"

	FIFOKind    = "fifo"
	SocketKind  = "socket"
	SymLinkKind = "symlink"
	BlkDevKind  = "blk"
	CharDevKind = "chr"
)

func IsGroup(k Kind) bool {
	switch k {
	case GroupKind, SmartGroupKind, ExternalGroupKind:
		return true
	}
	return false
}

func FileKind(filename string, defaultKind Kind) Kind {
	ext := strings.TrimPrefix(filepath.Ext(filename), ".")
	switch ext {
	case "txt":
		return TextKind

	case "md", "markdown":
		return FmtDocKind

	case "pdf":
		return PdfDocKind

	case "webarchive":
		return WebArchiveKind

	case "url":
		return WebBookmarkKind

	case "html", "htm":
		return HtmlKind

	case "doc", "docx", "pages":
		return WordDocKind

	case "xls", "xlsx", "numbers":
		return ExcelDocKind

	case "ppt", "pptx", "key":
		return PptDocKind

	case "jpg", "jpeg", "png", "bmp", "tif", "tiff", "gif", "heic", "heif":
		return ImageKind

	case "mp4", "avi", "mov", "wmv", "flv", "mkv", "mpeg", "mpg", "rm":
		return VideoKind

	case "mp3", "wav", "aac", "flac", "wma", "alac":
		return AudioKind

	default:
		return defaultKind
	}
}
