package session

import "bytes"

type terminalQuery struct {
	query  []byte
	answer []byte
}

var standardQueries = []terminalQuery{
	{[]byte("\x1b[6n"), []byte("\x1b[1;1R")},
	{[]byte("\x1b[c"), []byte("\x1b[?1;2c")},
	{[]byte("\x1b[>c"), []byte("\x1b[>0;0;0c")},
	{[]byte("\x1b[>q"), []byte("\x1b[>0c")},
	{[]byte("\x1b]10;?\x1b\\"), []byte("\x1b]10;rgb:0000/0000/0000\x1b\\")},
	{[]byte("\x1b]11;?\x1b\\"), []byte("\x1b]11;rgb:ffff/ffff/ffff\x1b\\")},
}

type terminalResponder struct {
	writeFn func([]byte) (int, error)
	buf     []byte
}

func newTerminalResponder(writeFn func([]byte) (int, error)) *terminalResponder {
	return &terminalResponder{
		writeFn: writeFn,
		buf:     make([]byte, 0, 4096),
	}
}

func (r *terminalResponder) feed(data []byte) []byte {
	r.buf = append(r.buf, data...)

	for _, q := range standardQueries {
		if idx := bytes.Index(r.buf, q.query); idx >= 0 {
			r.writeFn(q.answer)
		}
	}

	if len(r.buf) > 65536 {
		r.buf = r.buf[len(r.buf)-32768:]
	}

	return data
}
