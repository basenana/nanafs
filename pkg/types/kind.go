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

// Kind of Entry
type Kind string

const (
	/*
		system-wide kind
	*/
	GroupKind            = "group"
	SmartGroupKind       = "smtgroup"
	ExternalGroupKind    = "extgroup"
	GroupKindMap         = 0x02000
	SmartGroupKindMap    = 0x02001
	ExternalGroupKindMap = 0x02002

	/*
		text based file kind
	*/
	TextKind    = "text"
	TextKindMap = 0x01001

	/*
		format doc kind
	*/
	FmtDocKind    = "fmtdoc"
	FmtDocKindMap = 0x01002

	/*
		media file kind
	*/
	ImageKind    = "image"
	VideoKind    = "video"
	AudioKind    = "audio"
	ImageKindMap = 0x01003
	VideoKindMap = 0x01004
	AudioKindMap = 0x01005

	/*
		web based file kind
	*/
	WebArchiveKind    = "web"
	WebArchiveKindMap = 0x01006

	/*
		ungrouped files
	*/
	RawKind    = "raw"
	RawKindMap = 0x01000

	FIFOKind       = "fifo"
	SocketKind     = "socket"
	SymLinkKind    = "symlink"
	BlkDevKind     = "blk"
	CharDevKind    = "chr"
	FIFOKindMap    = 0x03001
	SocketKindMap  = 0x03002
	SymLinkKindMap = 0x03003
	BlkDevKindMap  = 0x03004
	CharDevKindMap = 0x03005
)

func IsGroup(k Kind) bool {
	switch k {
	case GroupKind, SmartGroupKind, ExternalGroupKind:
		return true
	}
	return false
}

var kindMap = map[Kind]int64{
	RawKind:           RawKindMap,
	GroupKind:         GroupKindMap,
	SmartGroupKind:    SmartGroupKindMap,
	ExternalGroupKind: ExternalGroupKindMap,
	FIFOKind:          FIFOKindMap,
	SocketKind:        SocketKindMap,
	SymLinkKind:       SymLinkKindMap,
	BlkDevKind:        BlkDevKindMap,
	CharDevKind:       CharDevKindMap,
}

func KindMap(k Kind) int64 {
	return kindMap[k]
}
