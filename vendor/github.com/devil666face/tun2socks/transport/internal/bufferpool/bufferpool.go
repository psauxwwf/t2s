package bufferpool

import (
	"bytes"

	"github.com/devil666face/tun2socks/internal/pool"
)

const _size = 1024 // by default, create 1 KiB buffers

var _pool = pool.New(func() *bytes.Buffer {
	return bytes.NewBuffer(make([]byte, 0, _size))
})

func Get() *bytes.Buffer {
	buf := _pool.Get()
	buf.Reset()
	return buf
}

func Put(b *bytes.Buffer) {
	_pool.Put(b)
}
