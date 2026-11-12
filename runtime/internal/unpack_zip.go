package internal

import (
	"bufio"
	"compress/flate"
	"encoding/binary"
	"hash/crc32"
	"io"
	"os"
)

type unpackByteReader struct {
	r io.Reader
	n int64
}

func (r *unpackByteReader) Read(p []byte) (int, error) {
	n, e := r.r.Read(p)
	r.n += int64(n)
	return n, e
}
func (r *unpackByteReader) ReadByte() (byte, error) {
	var b [1]byte
	_, e := io.ReadFull(r, b[:])
	return b[0], e
}
func unpackZIP(f *os.File, size int64, p *unpackCollector) error {
	if size < 22 {
		return errUnpackArchive
	}
	var end [22]byte
	if _, e := f.ReadAt(end[:], size-22); e != nil {
		return e
	}
	u16 := binary.LittleEndian.Uint16
	u32 := binary.LittleEndian.Uint32
	if u32(end[:]) != 0x06054b50 || u16(end[4:]) != 0 || u16(end[6:]) != 0 || u16(end[20:]) != 0 {
		return errUnpackArchive
	}
	count := int(u16(end[10:]))
	cdSize := int64(u32(end[12:]))
	cdOffset := int64(u32(end[16:]))
	if count < 1 || count > 4096 || int(u16(end[8:])) != count || cdSize < 46 || cdSize > 8<<20 || cdOffset < 0 || cdOffset+cdSize != size-22 {
		return errUnpackArchive
	}
	cd := io.NewSectionReader(f, cdOffset, cdSize)
	var next int64
	for i := 0; i < count; i++ {
		var h [46]byte
		if _, e := io.ReadFull(cd, h[:]); e != nil {
			return e
		}
		nameLen := int(u16(h[28:]))
		compressed := int64(u32(h[20:]))
		plain := int64(u32(h[24:]))
		offset := int64(u32(h[42:]))
		if u32(h[:]) != 0x02014b50 || u16(h[4:]) != 20 || u16(h[6:]) != 20 || u16(h[8:]) != 0 || u16(h[10:]) != 8 || nameLen < 1 || nameLen > 1024 || u16(h[30:]) != 0 || u16(h[32:]) != 0 || u16(h[34:]) != 0 || u16(h[36:]) != 0 || u32(h[38:]) != 0 || plain > unpackFileLimit || offset != next {
			return errUnpackArchive
		}
		name := make([]byte, nameLen)
		if _, e := io.ReadFull(cd, name); e != nil {
			return e
		}
		if !unpackName(string(name)) {
			return errUnpackArchive
		}
		var local [30]byte
		if _, e := f.ReadAt(local[:], offset); e != nil {
			return e
		}
		if u32(local[:]) != 0x04034b50 || u16(local[4:]) != 20 || u16(local[6:]) != 0 || u16(local[8:]) != 8 || u16(local[10:]) != u16(h[12:]) || u16(local[12:]) != u16(h[14:]) || u32(local[14:]) != u32(h[16:]) || u32(local[18:]) != uint32(compressed) || u32(local[22:]) != uint32(plain) || int(u16(local[26:])) != nameLen || u16(local[28:]) != 0 {
			return errUnpackArchive
		}
		localName := make([]byte, nameLen)
		if _, e := f.ReadAt(localName, offset+30); e != nil || string(localName) != string(name) {
			return errUnpackArchive
		}
		start := offset + 30 + int64(nameLen)
		next = start + compressed
		if next > cdOffset {
			return errUnpackArchive
		}
		source := &unpackByteReader{r: bufio.NewReader(io.NewSectionReader(f, start, compressed))}
		decoded := flate.NewReader(source)
		crc := crc32.NewIEEE()
		mode := os.FileMode(0644)
		if string(name) == "system/bin/ocmi" || string(name) == "system/bin/ocmi.exe" {
			mode = 0755
		}
		err := p.file(string(name), plain, mode, io.TeeReader(decoded, crc))
		closeErr := decoded.Close()
		if err != nil {
			return err
		}
		if closeErr != nil || source.n != compressed || crc.Sum32() != u32(h[16:]) {
			return errUnpackArchive
		}
	}
	if next != cdOffset {
		return errUnpackArchive
	}
	var extra [1]byte
	if n, e := cd.Read(extra[:]); n != 0 || e != io.EOF {
		return errUnpackArchive
	}
	return nil
}
