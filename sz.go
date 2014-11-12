// Package sz is an immature implementation of a Reader/Writer for the
// Snappy framing format.  Use at your own risk; bug reports and test cases
// are welcome.
package sz

import (
	"code.google.com/p/snappy-go/snappy" // native Go
	//"github.com/twotwotwo/sz/snappy" // cgo
	"errors"
	"hash"
	"hash/crc32"
	"io"
)

var ErrFramingCorrupt = errors.New("sz: snappy framing corrupt")
var ErrUnexpectedChunk = errors.New("sz: unexpected chunk type")
var ErrCRCMismatch = errors.New("sz: CRC mismatch")
var ErrStrictMemLimitExceeded = errors.New("sz: oversized block in strict-memory-usage mode")

var signature = []byte{0xff, 0x06, 0x00, 0x00, 0x73, 0x4e, 0x61, 0x50, 0x70, 0x59}
var castagnoli = crc32.MakeTable(crc32.Castagnoli)

const blockSize = 1 << 16

type Writer struct {
	w   io.Writer
	b   [blockSize]byte // uncompressed data
	c   []byte          // compressed data + framing
	n   int             // uncompressed byte count
	crc hash.Hash32
}

func NewWriter(w io.Writer) (*Writer, error) {
	_, err := w.Write(signature)
	if err != nil {
		return nil, err
	}
	return &Writer{
		w:   w,
		c:   make([]byte, snappy.MaxEncodedLen(65536)+8),
		crc: crc32.New(castagnoli),
	}, nil
}

// Writer accumulates up to 64KB of input, then compresses and flushes it.
func (w *Writer) Write(data []byte) (n int, err error) {
	for len(data) > 0 {
		copied := copy(w.b[w.n:], data)
		n += copied
		w.n += copied
		if w.n == blockSize {
			err = w.Flush()
			if err != nil {
				return
			}
		}
		data = data[copied:]
	}
	return
}

func (w *Writer) Close() error {
	return w.Flush()
}

// Flush ends the block and writes it out. It does not call any Flush
// method on the underlying Writer. Snappy does not keep context across
// blocks, so flushing frequently will hurt your compression ratio.
func (w *Writer) Flush() error {
	content, err := snappy.Encode(w.c[8:], w.b[:w.n])
	if err != nil {
		return err
	}
	head := w.c[:8]
	n := len(content)

	// substitute unpacked content if Snappy grew the block
	incompressible := n > w.n
	if incompressible {
		head[0] = 0x01
		// RF: with 8 extra bytes before w.b, incompressible
		// case could be a single Write too.
		content, n = w.b[:w.n], w.n
	} else {
		head[0] = 0x00
	}

	n += 4 // changing n from text length to *chunk* length incl CRC
	head[1], head[2], head[3] = byte(n), byte(n>>8), byte(n>>16)

	// checksum the uncompressed content
	w.crc.Reset()
	w.crc.Write(w.b[:w.n])
	sum := w.crc.Sum32()
	masked := ((sum >> 15) | (sum << 17)) + 0xa282ead8
	head[4], head[5], head[6], head[7] = byte(masked), byte(masked>>8), byte(masked>>16), byte(masked>>24)

	// Because we allocate MaxEncodedLen+8 up front, snappy.Encode
	// should never need a new slice, so w.c[:n+4] should already
	// be head followed by content, so we can make a single Write
	// unless we had incompressible input. 
	isContiguous := !incompressible && cap(w.c) >= n+4 && &w.c[8] == &content[0]
	if isContiguous {
		_, err = w.w.Write(w.c[:n+4])
		if err != nil {
			return err
		}
	} else {
		_, err = w.w.Write(head[:])
		if err != nil {
			return err
		}
		_, err = w.w.Write(content)
		if err != nil {
			return err
		}
	}
	w.n = 0 // reset uncompressed length for next time
	return err
}

// Reader reads content in the Snappy framing format.
type Reader struct {
	r      io.Reader
	blk    []byte          // raw block
	dec    []byte          // unread part of decompressed (or just copied) block
	spc    [blockSize]byte // space from which dec comes
	crc    hash.Hash32
	strict bool
}

func NewReader(r io.Reader) (*Reader, error) {
	var firstBytes [10]byte
	_, err := io.ReadFull(r, firstBytes[:])
	if err != nil {
		if err == io.EOF {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}
	for i, b := range firstBytes {
		if b != signature[i] {
			return nil, ErrFramingCorrupt
		}
	}
	return &Reader{
		r:   r,
		crc: crc32.New(castagnoli),
	}, nil
}

// NewReaderStrictMem returns a Reader that rejects any chunk that's
// larger than a Snappy compressor would produce even for worst-case
// incompressible content. This is against the framing format spec
// (which permits 16MB chunks), but prevents a specially constructed
// stream from using a lot of memory, and may be useful when consuming
// untrusted content.
func NewReaderStrictMem(r io.Reader) (*Reader, error) {
	szr, err := NewReader(r)
	szr.strict = true
	return szr, err
}

var strictBlockLimit = snappy.MaxEncodedLen(65536) + 8

const (
	chunkTypeCompressed     = 0x00
	chunkTypeUncompressed   = 0x01
	chunkTypeFirstSkippable = 0x80
	chunkTypePadding        = 0xfe
	chunkTypeSig            = 0xff
)

func (r *Reader) Read(p []byte) (n int, err error) {
	// while there is anything left to satisfy
	for err == nil && n < len(p) {
		// first satisfy from current block
		if len(r.dec) > 0 {
			copied := copy(p[n:], r.dec)
			n += copied
			r.dec = r.dec[copied:]
			continue
		}

		// then read another block head
		var head [8]byte
		_, err = io.ReadFull(r.r, head[:4])
		if err != nil {
			// io.EOF here is expected; pass through
			return
		}
		tag := head[0]
		l := int(head[1]) + int(head[2])<<8 + int(head[3])<<16
		if r.strict && l > strictBlockLimit {
			err = ErrStrictMemLimitExceeded
			return
		}
		if cap(r.blk) < l {
			r.blk = make([]byte, l)
		}
		r.blk = r.blk[:l]
		_, err = io.ReadFull(r.r, r.blk)
		if err != nil {
			if err == io.EOF {
				err = io.ErrUnexpectedEOF
			}
			return
		}

		// extract crc from block types that have it
		masked := uint32(0)
		if tag == chunkTypeCompressed || tag == chunkTypeUncompressed {
			if len(r.blk) < 4 {
				err = ErrFramingCorrupt
				return
			}
			masked = uint32(r.blk[0]) + uint32(r.blk[1])<<8 + uint32(r.blk[2])<<16 + uint32(r.blk[3])<<24
		}

		switch tag {
		case chunkTypeCompressed:
			var decodedLen int
			decodedLen, err = snappy.DecodedLen(r.blk[4:])
			if err != nil || decodedLen > blockSize {
				err = ErrFramingCorrupt
				return
			}
			r.dec, err = snappy.Decode(r.spc[:], r.blk[4:])
			if err != nil {
				return
			}
		case chunkTypeUncompressed:
			if l > blockSize+4 {
				// only 64k+CRC allowed
				err = ErrFramingCorrupt
				return
			}
			copied := copy(r.spc[:], r.blk[4:])
			r.dec = r.spc[:copied]
		case chunkTypeSig:
			// check for expected signature
			if l != 6 {
				err = ErrFramingCorrupt
				return
			}
			for i, b := range r.blk {
				if b != signature[i+4] {
					err = ErrFramingCorrupt
					return
				}
			}
		default:
			if tag < chunkTypeFirstSkippable {
				err = ErrUnexpectedChunk
				return
			}
			// else skippable tag; no more to do
			// this includes 0xfe, padding
		}
		if tag == chunkTypeCompressed || tag == chunkTypeUncompressed {
			// check crc
			r.crc.Reset()
			r.crc.Write(r.dec)
			sum := r.crc.Sum32()
			actualMasked := ((sum >> 15) | (sum << 17)) + 0xa282ead8
			if actualMasked != masked {
				err = ErrCRCMismatch
				return
			}
		}
	}
	return
}
