package localprocess

import "bytes"

type limitedBuffer struct {
	limit     int64
	buf       bytes.Buffer
	truncated bool
}

func newLimitedBuffer(limit int64) *limitedBuffer {
	return &limitedBuffer{limit: limit}
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.limit < 0 {
		b.limit = 0
	}
	before := int64(b.buf.Len())
	if before < b.limit {
		remaining := b.limit - int64(b.buf.Len())
		toWrite := p
		if int64(len(toWrite)) > remaining {
			toWrite = toWrite[:remaining]
		}
		_, _ = b.buf.Write(toWrite)
	}
	if before+int64(len(p)) > b.limit {
		b.truncated = true
	}
	return len(p), nil
}

func (b *limitedBuffer) Bytes() []byte {
	return append([]byte(nil), b.buf.Bytes()...)
}

func (b *limitedBuffer) Truncated() bool {
	return b.truncated
}
