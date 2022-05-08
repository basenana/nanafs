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
	fs.FileReader
	fs.FileWriter
	fs.FileFlusher
	fs.FileFsyncer
	fs.FileReleaser
}
