// Copyright 2011 The Snappy-Go Authors. All rights reserved.
// Copyright 2014 Randall Farmer.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package snappy

import (
	"encoding/binary"
	"unsafe"
)

// What follows is a port of most of the snappy-go encoder "back" to C, with
// only surface-level tweaks to use pointers, etc.  Compared to the
// impressive full C++ Snappy implementation, it is slower but also, I'm
// hoping, more understandable and hackable.

/*
#cgo CFLAGS: -O3

#include <string.h>
#include <stdint.h>

#define HASH_BITS 14
#define HASH_SHIFT (32-HASH_BITS)
#define MAXOFFSET (1<<15)
#define TAG_LITERAL 0
#define TAG_COPY1 1
#define TAG_COPY2 2
#define TAG_COPY4 3

#if defined(__i386__) || defined(__x86_64__) || defined(_M_IX86) || defined(_M_X64) || defined(__ppc__) || defined(__ppc64__)
# define L32(p) (*((uint32_t*)(p)))
#else
# define L8(v) ((uint32_t)(v&0xFF))
# define L32(p) (L8((p)[0]) | (L8((p)[1]) << 8) | (L8((p)[2]) << 16) | (L8((p)[3]) << 24))
#endif

// emitCopy writes a copy element between d and dEnd, returning a new d,
// or can return NULL if it seems there might not be enough space.
char* emitCopy(char* d, char* dEnd, int offset, int len) {
    while (len>0) {
        if (d > dEnd-3) return NULL;
        int l = len-4;
        if (0<=l && l<1<<3 && offset < 1<<11) {
            *d++ = ((offset>>8)&0x07)<<5 | l<<2 | TAG_COPY1;
            *d++ = offset;
            break;
        }

        l=len;
        if (l>64) l=64;
        *d++ = (l-1)<<2 | TAG_COPY2;
        *d++ = offset;
        *d++ = offset >> 8;
        len -= l;
    }
    return d;
}

// emitLiteral writes a literal element between d and dEnd, returning
// a new d, or can return NULL if it seems there might not be space.
char* emitLiteral(char* d, char* dEnd, char* start, char* end) {
    int len = end-start;
    int l = len-1;
    if (d > dEnd - 6 - len) return NULL;
    if (len<60) {
        *d++ = l<<2 | TAG_LITERAL;
    } else if (len < 256) {
        *d++ = 60<<2 | TAG_LITERAL;
        *d++ = l;
    } else if (len < 1<<16) {
        *d++ = 61<<2 | TAG_LITERAL;
        *d++ = l;
        *d++ = l>>8;
    } else if (len < 1<<24) {
        *d++ = 62<<2 | TAG_LITERAL;
        *d++ = l;
        *d++ = l>>8;
        *d++ = l>>16;
    } else if (len < 1L<<32) {
        *d++ = 63<<2 | TAG_LITERAL;
        *d++ = l;
        *d++ = l>>8;
        *d++ = l>>16;
        *d++ = l>>24;
    }
    memcpy(d, start, len);
    d += len;
    return d;
}

// encode writes Snappy-encoded content between d and dEnd, returning the
// compressed length.  It may return -1 if the compressed content was too
// large to fit in d or nearly so.
int encode(char* d, char* dEnd, char* s, char* sEnd) {
    char* tbl[1<<HASH_BITS];
    int i;
    for (i = 0; i < 1<<HASH_BITS; i++)
        tbl[i] = s-1;
    char* lit = s;
    char* dOrig = d;
    char* sOrig = s;
    while (s+3 < sEnd) {
        uint32_t h = L32(s);
        char** p = tbl + ((h*0x1e35a7bd) >> HASH_SHIFT);
        char* t = *p;
        *p = s;
        if (t<sOrig || s-t > MAXOFFSET || h != L32(t)) {
            s++;
            continue;
        }
        char* s0 = s;
	s+=4; t+=4;
        while (s<sEnd && *s==*t) ++s, ++t;
        if (lit!=s0) d = emitLiteral(d, dEnd, lit, s0);
        if (d==NULL) return -1;
        d = emitCopy(d, dEnd, s-t, s-s0);
        if (d==NULL) return -1;
        lit = s;
    }
    if (lit!=sEnd) d = emitLiteral(d, dEnd, lit, sEnd);
    if (d==NULL) return -1;
    return d-dOrig;
}
*/
import "C"

// Encode returns the encoded form of src. The returned slice may be a sub-
// slice of dst if dst was large enough to hold the entire encoded block.
// Otherwise, a newly allocated slice will be returned.
// It is valid to pass a nil dst.
func Encode(dst, src []byte) ([]byte, error) {
	if n := MaxEncodedLen(len(src)); len(dst) < n {
		dst = make([]byte, n)
	}

	// The block starts with the varint-encoded length of the decompressed bytes.
	d := binary.PutUvarint(dst, uint64(len(src)))
	if len(src) == 0 {
		return dst[:d], nil
	}

	dp := uintptr(unsafe.Pointer(&dst[0]))
	sp := uintptr(unsafe.Pointer(&src[0]))
	l := C.encode(
		(*C.char)(unsafe.Pointer(dp+uintptr(d))), 
        	(*C.char)(unsafe.Pointer(dp+uintptr(len(dst)))), 
		(*C.char)(unsafe.Pointer(sp)), 
		(*C.char)(unsafe.Pointer(sp+uintptr(len(src)))),
	)
	if l<0 {
		panic("destination buffer too small")
	}
	return dst[:d+int(l)], nil
}

// MaxEncodedLen returns the maximum length of a snappy block, given its
// uncompressed length.
func MaxEncodedLen(srcLen int) int {
	// this is a formula inherited from snappy-go, which inherits it
	// from the C++ code, which explains that 5-byte-long copy elements
	// alternating with one-byte literals can cause a 7/6 blowup. 
	// however, *this* encoder never produces 5-byte copies, so I think
	// *its* max blowup is more like 66/65 + preamble.  
	// 
	// leaving it alone for now out of general paranoia, especially
	// where C code is involved.
	return 32 + srcLen + srcLen/6
}
