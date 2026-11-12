// Package finder reads, never writes, what the user set in Finder: the
// sidebar's favorites (the sfl4/sfl3 shared file lists) and where a new
// window opens. It is plain Go with only the standard library: a small
// binary plist reader, an NSKeyedArchiver unarchiver and a bookmark
// reader.
package finder

import (
	"encoding/binary"
	"errors"
	"math"
	"unicode/utf16"
)

// UID is a reference into an NSKeyedArchiver's $objects.
type UID uint64

var errPlist = errors.New("finder: bad binary plist")

// parsePlist decodes a binary plist (bplist00) into nil, bool, int64,
// float64, string, []byte, UID, []any and map[string]any. Dates come as
// float64 seconds since 2001.
func parsePlist(b []byte) (any, error) {
	if len(b) < 8+32 || string(b[:8]) != "bplist00" {
		return nil, errPlist
	}
	tr := b[len(b)-32:]
	offSize, refSize := int(tr[6]), int(tr[7])
	n := binary.BigEndian.Uint64(tr[8:])
	top := binary.BigEndian.Uint64(tr[16:])
	table := binary.BigEndian.Uint64(tr[24:])
	if offSize < 1 || offSize > 8 || refSize < 1 || refSize > 8 || n == 0 || top >= n ||
		table >= uint64(len(b)) || n > uint64(len(b)) || table+n*uint64(offSize) > uint64(len(b)-32) {
		return nil, errPlist
	}
	p := &plist{b: b, refSize: refSize, offs: make([]int, n), seen: make([]bool, n)}
	for i := range p.offs {
		o := beUint(b[int(table)+i*offSize:][:offSize])
		if o >= uint64(len(b)-32) {
			return nil, errPlist
		}
		p.offs[i] = int(o)
	}
	return p.object(top)
}

type plist struct {
	b       []byte
	refSize int
	offs    []int
	seen    []bool // objects being decoded, against cycles
}

func beUint(b []byte) uint64 {
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return v
}

func (p *plist) object(ref uint64) (any, error) {
	if ref >= uint64(len(p.offs)) || p.seen[ref] {
		return nil, errPlist
	}
	p.seen[ref] = true
	defer func() { p.seen[ref] = false }()
	o := p.offs[ref]
	m := p.b[o]
	hi, lo := m>>4, int(m&0xf)
	at := o + 1
	// count reads the length in the low nibble, or the int after it.
	count := func() (int, error) {
		if lo != 0xf {
			return lo, nil
		}
		if at >= len(p.b) || p.b[at]>>4 != 1 {
			return 0, errPlist
		}
		w := 1 << (p.b[at] & 0xf)
		if at+1+w > len(p.b) {
			return 0, errPlist
		}
		v := beUint(p.b[at+1 : at+1+w])
		at += 1 + w
		if v > uint64(len(p.b)) {
			return 0, errPlist
		}
		return int(v), nil
	}
	need := func(k int) error {
		if k < 0 || at+k > len(p.b) {
			return errPlist
		}
		return nil
	}
	switch hi {
	case 0:
		switch m {
		case 0x08:
			return false, nil
		case 0x09:
			return true, nil
		}
		return nil, nil
	case 1:
		w := 1 << lo
		if err := need(w); err != nil || w > 16 {
			return nil, errPlist
		}
		v := p.b[at : at+w]
		if w == 16 {
			v = v[8:] // 128-bit ints: the low half
		}
		return int64(beUint(v)), nil
	case 2, 3:
		w := 1 << lo
		if m == 0x33 {
			w = 8
		}
		if err := need(w); err != nil {
			return nil, err
		}
		switch w {
		case 4:
			return float64(math.Float32frombits(binary.BigEndian.Uint32(p.b[at:]))), nil
		case 8:
			return math.Float64frombits(binary.BigEndian.Uint64(p.b[at:])), nil
		}
		return nil, errPlist
	case 4:
		k, err := count()
		if err != nil || need(k) != nil {
			return nil, errPlist
		}
		return append([]byte(nil), p.b[at:at+k]...), nil
	case 5:
		k, err := count()
		if err != nil || need(k) != nil {
			return nil, errPlist
		}
		return string(p.b[at : at+k]), nil
	case 6:
		k, err := count()
		if err != nil || need(2*k) != nil {
			return nil, errPlist
		}
		u := make([]uint16, k)
		for i := range u {
			u[i] = binary.BigEndian.Uint16(p.b[at+2*i:])
		}
		return string(utf16.Decode(u)), nil
	case 8:
		if err := need(lo + 1); err != nil {
			return nil, err
		}
		return UID(beUint(p.b[at : at+lo+1])), nil
	case 0xa, 0xc:
		k, err := count()
		if err != nil || need(k*p.refSize) != nil {
			return nil, errPlist
		}
		arr := make([]any, k)
		for i := range arr {
			v, err := p.object(beUint(p.b[at+i*p.refSize:][:p.refSize]))
			if err != nil {
				return nil, err
			}
			arr[i] = v
		}
		return arr, nil
	case 0xd:
		k, err := count()
		if err != nil || need(2*k*p.refSize) != nil {
			return nil, errPlist
		}
		d := make(map[string]any, k)
		for i := range k {
			kv, err := p.object(beUint(p.b[at+i*p.refSize:][:p.refSize]))
			if err != nil {
				return nil, err
			}
			key, ok := kv.(string)
			if !ok {
				return nil, errPlist
			}
			v, err := p.object(beUint(p.b[at+(k+i)*p.refSize:][:p.refSize]))
			if err != nil {
				return nil, err
			}
			d[key] = v
		}
		return d, nil
	}
	return nil, errPlist
}
