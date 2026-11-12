package internal

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"strconv"
	"strings"
)

func unpackOctal(b []byte) (int64, error) {
	s := strings.Trim(string(b), " \x00")
	if s == "" {
		return 0, nil
	}
	if strings.Trim(s, "01234567") != "" {
		return 0, errUnpackArchive
	}
	return strconv.ParseInt(s, 8, 64)
}
func unpackTarText(b []byte) (string, error) {
	i := bytes.IndexByte(b, 0)
	if i < 0 {
		i = len(b)
	}
	if !unpackZero(b[i:]) {
		return "", errUnpackArchive
	}
	return string(b[:i]), nil
}
func unpackZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}
func unpackPAX(raw []byte) (string, error) {
	fields := map[string]string{}
	for len(raw) > 0 {
		space := bytes.IndexByte(raw, ' ')
		if space < 1 || space > 8 {
			return "", errUnpackArchive
		}
		n, e := strconv.Atoi(string(raw[:space]))
		if e != nil || n <= space+2 || n > len(raw) || raw[n-1] != '\n' {
			return "", errUnpackArchive
		}
		record := raw[space+1 : n-1]
		sep := bytes.IndexByte(record, '=')
		if sep < 1 {
			return "", errUnpackArchive
		}
		key, val := string(record[:sep]), string(record[sep+1:])
		if (key != "path" && key != "mtime") || fields[key] != "" || val == "" {
			return "", errUnpackArchive
		}
		fields[key] = val
		raw = raw[n:]
	}
	if len(fields) != 2 || !unpackName(fields["path"]) || len(fields["mtime"]) > 64 {
		return "", errUnpackArchive
	}
	s := fields["mtime"]
	if s[0] == '-' {
		s = s[1:]
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return "", errUnpackArchive
	}
	for _, v := range parts {
		if v == "" || strings.Trim(v, "0123456789") != "" {
			return "", errUnpackArchive
		}
	}
	return fields["path"], nil
}
func unpackTAR(f *os.File, size int64, p *unpackCollector) error {
	var header [10]byte
	if _, e := f.ReadAt(header[:], 0); e != nil {
		return e
	}
	if header[0] != 31 || header[1] != 139 || header[2] != 8 || header[3] != 0 {
		return errUnpackArchive
	}
	source := &unpackByteReader{r: bufio.NewReader(io.NewSectionReader(f, 0, size))}
	gz, e := gzip.NewReader(source)
	if e != nil {
		return e
	}
	defer gz.Close()
	gz.Multistream(false)
	stream := &io.LimitedReader{R: gz, N: unpackWireLimit + 1}
	var pending string
	headers := 0
	metadata := int64(0)
	for {
		var h [512]byte
		if _, e = io.ReadFull(stream, h[:]); e != nil {
			return e
		}
		if unpackZero(h[:]) {
			if pending != "" {
				return errUnpackArchive
			}
			var second [512]byte
			if _, e = io.ReadFull(stream, second[:]); e != nil || !unpackZero(second[:]) {
				return errUnpackArchive
			}
			// The producer emits exactly two canonical zero blocks, no surplus members.
			var last [1]byte
			if n, e := stream.Read(last[:]); n != 0 || e != io.EOF || stream.N <= 0 || source.n != size {
				return errUnpackArchive
			}
			return nil
		}
		headers++
		if headers > 8192 || stream.N <= 0 {
			return errUnpackArchive
		}
		stored, e := unpackOctal(h[148:156])
		if e != nil {
			return e
		}
		var sum int64
		for i, c := range h {
			if i >= 148 && i < 156 {
				sum += 32
			} else {
				sum += int64(c)
			}
		}
		if sum != stored || string(h[257:263]) != "ustar\x00" || string(h[263:265]) != "00" {
			return errUnpackArchive
		}
		mode, e := unpackOctal(h[100:108])
		if e != nil {
			return e
		}
		if mode != 0644 && mode != 0755 {
			return errUnpackArchive
		}
		major, e := unpackOctal(h[329:337])
		if e != nil || major != 0 {
			return errUnpackArchive
		}
		minor, e := unpackOctal(h[337:345])
		if e != nil || minor != 0 {
			return errUnpackArchive
		}
		uid, e := unpackOctal(h[108:116])
		if e != nil {
			return e
		}
		gid, e := unpackOctal(h[116:124])
		if e != nil {
			return e
		}
		length, e := unpackOctal(h[124:136])
		if e != nil || length < 0 {
			return errUnpackArchive
		}
		if _, e = unpackOctal(h[136:148]); e != nil {
			return e
		}
		name, e := unpackTarText(h[:100])
		if e != nil {
			return e
		}
		prefix, e := unpackTarText(h[345:500])
		if e != nil {
			return e
		}
		if prefix != "" {
			name = prefix + "/" + name
		}
		if uid != 0 || gid != 0 || !unpackZero(h[157:257]) || !unpackZero(h[265:329]) || !unpackZero(h[500:]) {
			return errUnpackArchive
		}
		if h[156] == 'x' {
			// .NET's actual PAX metadata label is not a payload path.
			label := strings.TrimSuffix(strings.TrimPrefix(name, "./PaxHeaders."), "/.")
			if name != "./PaxHeaders."+label+"/." || len(label) < 1 || len(label) > 20 || strings.Trim(label, "0123456789") != "" {
				return errUnpackArchive
			}
			if pending != "" || length < 1 || length > 16<<10 || metadata+length > 2<<20 {
				return errUnpackArchive
			}
			metadata += length
			raw := make([]byte, int(length))
			if _, e = io.ReadFull(stream, raw); e != nil {
				return e
			}
			pending, e = unpackPAX(raw)
			if e != nil {
				return e
			}
		} else if h[156] == '0' {
			if pending == "" || length > unpackFileLimit || (mode != 0644 && mode != 0755) || !unpackName(name) {
				return errUnpackArchive
			}
			r := &io.LimitedReader{R: stream, N: length}
			if e = p.file(pending, length, os.FileMode(mode), r); e != nil {
				return e
			}
			pending = ""
		} else {
			return errUnpackArchive
		}
		pad := (512 - length%512) % 512
		if pad > 0 {
			var b [512]byte
			if _, e = io.ReadFull(stream, b[:pad]); e != nil || !unpackZero(b[:pad]) {
				return errUnpackArchive
			}
		}
	}
}
