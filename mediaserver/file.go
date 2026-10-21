package mediaserver

import (
	"errors"
	"io"
	"io/fs"
	"sync"

	"github.com/hirochachacha/go-smb2"
)

const (
	blockSize  = 512 << 10
	keptBlocks = 8
)

type cachedFile struct {
	server *Server
	file   *smb2.File
	info   fs.FileInfo
	size   int64

	mu     sync.Mutex
	offset int64
	blocks map[int64][]byte
	order  []int64
}

func newCachedFile(server *Server, file *smb2.File, info fs.FileInfo) *cachedFile {
	return &cachedFile{
		server: server,
		file:   file,
		info:   info,
		size:   info.Size(),
		blocks: map[int64][]byte{},
	}
}

func (f *cachedFile) Name() string {
	return f.info.Name()
}

func (f *cachedFile) Stat() (fs.FileInfo, error) {
	return f.info, nil
}

func (f *cachedFile) block(index int64) ([]byte, error) {
	if data, found := f.blocks[index]; found {
		return data, nil
	}

	data := make([]byte, min(blockSize, f.size-index*blockSize))
	n, err := f.file.ReadAt(data, index*blockSize)
	if err != nil && !(errors.Is(err, io.EOF) && n == len(data)) {
		return nil, f.server.failed(err)
	}

	f.blocks[index] = data
	f.order = append(f.order, index)
	if len(f.order) > keptBlocks {
		delete(f.blocks, f.order[0])
		f.order = f.order[1:]
	}

	return data, nil
}

func (f *cachedFile) ReadAt(p []byte, offset int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.readAt(p, offset)
}

func (f *cachedFile) readAt(p []byte, offset int64) (int, error) {
	if offset >= f.size {
		return 0, io.EOF
	}

	read := 0
	for read < len(p) && offset < f.size {
		index := offset / blockSize
		data, err := f.block(index)
		if err != nil {
			return read, err
		}

		n := copy(p[read:], data[offset-index*blockSize:])
		read += n
		offset += int64(n)
	}

	if read < len(p) {
		return read, io.EOF
	}

	return read, nil
}

func (f *cachedFile) Read(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	n, err := f.readAt(p, f.offset)
	f.offset += int64(n)
	if errors.Is(err, io.EOF) && n > 0 {
		err = nil
	}

	return n, err
}

func (f *cachedFile) Seek(offset int64, whence int) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += f.offset
	case io.SeekEnd:
		offset += f.size
	default:
		return 0, errors.New("unknown seek")
	}
	if offset < 0 {
		return 0, errors.New("seek before the start of the file")
	}

	f.offset = offset
	return offset, nil
}

func (f *cachedFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.blocks = nil
	return f.file.Close()
}
