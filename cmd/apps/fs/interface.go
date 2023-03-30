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

package fs

import "github.com/hanwen/go-fuse/v2/fs"

type nodeOperation interface {
	fs.NodeOnAdder

	fs.NodeAccesser
	fs.NodeStatfser
	fs.NodeGetattrer
	fs.NodeSetattrer
	fs.NodeGetxattrer
	fs.NodeSetxattrer
	fs.NodeRemovexattrer
	fs.NodeOpener
	fs.NodeLookuper
	fs.NodeCreater
	fs.NodeOpendirer
	fs.NodeReaddirer
	fs.NodeMkdirer
	fs.NodeMknoder
	fs.NodeLinker
	fs.NodeSymlinker
	fs.NodeReadlinker
	fs.NodeUnlinker
	fs.NodeRmdirer
	fs.NodeRenamer
	fs.NodeReleaser
}

type fileOperation interface {
	fs.FileGetattrer
	fs.FileReader
	fs.FileWriter
	fs.FileFlusher
	fs.FileFsyncer
	fs.FileReleaser
}
