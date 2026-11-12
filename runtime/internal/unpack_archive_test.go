package internal

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func unpackTestHash(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }

// Neutral fixture uses the existing independent factory consumer. It contains
// the current test executable for this native target, never a private kit.
func unpackSyntheticArchives(t *testing.T) ([]byte, []byte) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "factory")
	acceptanceUpdateFactory(t, root, strings.Repeat("a", 40))
	var names []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			n, e := filepath.Rel(root, path)
			if e != nil {
				return e
			}
			names = append(names, filepath.ToSlash(n))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(names)
	var zipped, tar bytes.Buffer
	zw := zip.NewWriter(&zipped)
	header := func(name string, size int, mode int64, kind byte) {
		var h [512]byte
		copy(h[:100], name)
		copy(h[100:108], fmt.Sprintf("%07o\x00", mode))
		copy(h[108:116], "0000000\x00")
		copy(h[116:124], "0000000\x00")
		copy(h[124:136], fmt.Sprintf("%011o\x00", size))
		copy(h[136:148], "00000000000\x00")
		for i := 148; i < 156; i++ {
			h[i] = ' '
		}
		h[156] = kind
		copy(h[257:265], "ustar\x0000")
		var sum int
		for _, v := range h {
			sum += int(v)
		}
		copy(h[148:156], fmt.Sprintf("%06o\x00 ", sum))
		tar.Write(h[:])
	}
	payload := func(raw []byte) {
		tar.Write(raw)
		if pad := (512 - len(raw)%512) % 512; pad > 0 {
			tar.Write(make([]byte, pad))
		}
	}
	pax := func(key, val string) string {
		body := " " + key + "=" + val + "\n"
		n := len(body) + 1
		for {
			s := fmt.Sprint(n) + body
			if len(s) == n {
				return s
			}
			n = len(s)
		}
	}
	for _, name := range names {
		raw, e := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if e != nil {
			t.Fatal(e)
		}
		var compressed bytes.Buffer
		fl, e := flate.NewWriter(&compressed, flate.BestSpeed)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = fl.Write(raw); e != nil {
			t.Fatal(e)
		}
		if e = fl.Close(); e != nil {
			t.Fatal(e)
		}
		h := &zip.FileHeader{Name: name, CreatorVersion: 20, ReaderVersion: 20, Method: zip.Deflate, CRC32: crc32.ChecksumIEEE(raw), CompressedSize64: uint64(compressed.Len()), UncompressedSize64: uint64(len(raw))}
		out, e := zw.CreateRaw(h)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = out.Write(compressed.Bytes()); e != nil {
			t.Fatal(e)
		}
		metadata := []byte(pax("path", name) + pax("mtime", "0"))
		header("./PaxHeaders.1/.", len(metadata), 0644, 'x')
		payload(metadata)
		mode := int64(0644)
		if name == "system/bin/ocmi" || name == "system/bin/ocmi.exe" {
			mode = 0755
		}
		header(name, len(raw), mode, '0')
		payload(raw)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	tar.Write(make([]byte, 1024))
	var gzipped bytes.Buffer
	gz := gzip.NewWriter(&gzipped)
	if _, err := gz.Write(tar.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return zipped.Bytes(), gzipped.Bytes()
}
func TestWorkerUnpackClosedFixtures(t *testing.T) {
	zipBytes, tarBytes := unpackSyntheticArchives(t)
	var original unpackArchiveInfo
	for i, raw := range [][]byte{zipBytes, tarBytes} {
		path := filepath.Join(t.TempDir(), "fixture")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		f, e := os.Open(path)
		if e != nil {
			t.Fatal(e)
		}
		defer f.Close()
		info, e := validateUnpackArchive(f, unpackTestHash(raw))
		if e != nil {
			t.Fatalf("closed fixture %d: %v", i, e)
		}
		if len(info.Entries) < 9 || info.SourceCommit != strings.Repeat("a", 40) || info.Target != runtime.GOOS+"-"+runtime.GOARCH {
			t.Fatal("actual inventory", info.SourceCommit, len(info.Entries))
		}
		var total int64
		err := streamUnpackArchive(f, unpackTestHash(raw), info, func(e unpackEntry, r io.Reader) error { n, err := io.Copy(io.Discard, r); total += n; return err })
		var want int64
		for _, e := range info.Entries {
			want += e.Size
		}
		if err != nil || total != want {
			t.Fatal("actual streaming", err, total)
		}
		if i == 0 {
			original = info
		} else if !reflect.DeepEqual(original, info) {
			t.Fatal("ZIP/PAX inventory differs")
		}
		if streamUnpackArchive(f, unpackTestHash(raw), info, func(e unpackEntry, r io.Reader) error { return nil }) == nil {
			t.Fatal("unconsumed sink accepted")
		}
	}
}
func TestWorkerUnpackNames(t *testing.T) {
	for _, n := range []string{"../escape", "system/../escape", "system/a\\b", "system/CON.txt", "system/com1", "system/NUL", "system/a.", "system/a ", "system/é", "system/a:b", "local/x", "system/.git/x", "/system/x", "system//x", "system/"} {
		if unpackName(n) {
			t.Fatal("unsafe name", n)
		}
	}
	for _, n := range []string{"system/VERSION.md", ".ocmi-home.json", "SHA256SUMS"} {
		if !unpackName(n) {
			t.Fatal("valid name refused", n)
		}
	}
	for _, pair := range [][2]string{{"system/a", "system/A"}, {"system/a", "system/a/b"}, {"system/a/b", "system/a"}, {"system/Folder/a", "system/folder/b"}} {
		p := unpackCollector{names: map[string]bool{}, metadata: map[string][]byte{}}
		if err := p.file(pair[0], 0, 0644, strings.NewReader("")); err != nil {
			t.Fatal(err)
		}
		if err := p.file(pair[1], 0, 0644, strings.NewReader("")); err == nil {
			t.Fatal("alias/prefix accepted", pair)
		}
	}
	for _, pair := range [][2]string{{"system/Folder/a", "system/Folders/b"}, {"system/Folder/a", "system/Other/b"}, {"system/Folder/a", "system/Folder/b"}} {
		p := unpackCollector{names: map[string]bool{}, metadata: map[string][]byte{}}
		for _, n := range pair {
			if err := p.file(n, 0, 0644, strings.NewReader("")); err != nil {
				t.Fatal("distinct/canonical directories refused", pair, err)
			}
		}
	}
}
func TestWorkerUnpackMatchingDigestRefusals(t *testing.T) {
	raw, _ := unpackSyntheticArchives(t)
	cd := int(binary.LittleEndian.Uint32(raw[len(raw)-6:]))
	cases := map[string]func([]byte) []byte{
		"zip64":         func(b []byte) []byte { binary.LittleEndian.PutUint16(b[len(b)-12:], 65535); return b },
		"encryption":    func(b []byte) []byte { b[6] = 1; return b },
		"central-extra": func(b []byte) []byte { b[cd+30] = 1; return b },
		"central-size":  func(b []byte) []byte { binary.LittleEndian.PutUint32(b[cd+24:], uint32(unpackFileLimit+1)); return b },
		"zero-crc": func(b []byte) []byte {
			binary.LittleEndian.PutUint32(b[14:], 0)
			binary.LittleEndian.PutUint32(b[cd+16:], 0)
			return b
		},
		"unsafe-name": func(b []byte) []byte { b[30] = ':'; b[cd+46] = ':'; return b },
		"trailing":    func(b []byte) []byte { return append(b, 0) },
		"truncated":   func(b []byte) []byte { return b[:len(b)-1] },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			b := mutate(append([]byte(nil), raw...))
			file := filepath.Join(t.TempDir(), "bad.zip")
			if e := os.WriteFile(file, b, 0600); e != nil {
				t.Fatal(e)
			}
			f, e := os.Open(file)
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			if _, e = validateUnpackArchive(f, unpackTestHash(b)); e == nil {
				t.Fatal("matching digest bypassed parser")
			}
		})
	}
	if _, e := unpackPAX([]byte("999999999999999 path=system/x\n")); e == nil {
		t.Fatal("unbounded PAX length")
	}
	if _, e := unpackPAX([]byte("17 path=../escape\n")); e == nil {
		t.Fatal("unsafe PAX")
	}
	p := unpackCollector{names: map[string]bool{}, metadata: map[string][]byte{}}
	if p.file("system/a", 1, 0644, strings.NewReader("xx")) == nil {
		t.Fatal("size overflow")
	}
}

func TestWorkerUnpackPAXGzipRefusals(t *testing.T) {
	_, raw := unpackSyntheticArchives(t)
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	tar, err := io.ReadAll(io.LimitReader(gz, unpackWireLimit+1))
	gz.Close()
	if err != nil || int64(len(tar)) > unpackWireLimit {
		t.Fatal(err)
	}
	repack := func(tar []byte) []byte {
		var b bytes.Buffer
		z := gzip.NewWriter(&b)
		if _, e := z.Write(tar); e != nil {
			t.Fatal(e)
		}
		if e := z.Close(); e != nil {
			t.Fatal(e)
		}
		return b.Bytes()
	}
	checksum := func(h []byte) {
		for i := 148; i < 156; i++ {
			h[i] = ' '
		}
		var sum int
		for _, b := range h[:512] {
			sum += int(b)
		}
		v := fmt.Sprintf("%06o\x00 ", sum)
		copy(h[148:156], v)
	}
	cases := map[string]func() []byte{
		"gzip-crc":           func() []byte { b := append([]byte(nil), raw...); b[len(b)-8] ^= 1; return b },
		"gzip-second-member": func() []byte { return append(append([]byte(nil), raw...), raw...) },
		"gzip-trailing":      func() []byte { return append(append([]byte(nil), raw...), 0) },
		"tar-special":        func() []byte { b := append([]byte(nil), tar...); b[156] = 'g'; checksum(b[:512]); return repack(b) },
		"pax-oversize": func() []byte {
			b := append([]byte(nil), tar...)
			copy(b[124:136], []byte("00000100001\x00"))
			checksum(b[:512])
			return repack(b)
		},
		"pax-unknown": func() []byte {
			b := append([]byte(nil), tar...)
			end := bytes.Index(b[512:], []byte("path="))
			if end < 0 {
				t.Fatal("actual PAX premise")
			}
			copy(b[512+end:], "evil=")
			return repack(b)
		},
		"tar-trailer": func() []byte { return repack(tar[:len(tar)-512]) },
	}
	for name, makeBad := range cases {
		t.Run(name, func(t *testing.T) {
			b := makeBad()
			path := filepath.Join(t.TempDir(), "bad.tar.gz")
			if e := os.WriteFile(path, b, 0600); e != nil {
				t.Fatal(e)
			}
			f, e := os.Open(path)
			if e != nil {
				t.Fatal(e)
			}
			defer f.Close()
			if _, e = validateUnpackArchive(f, unpackTestHash(b)); e == nil {
				t.Fatal("matching digest bypassed structure")
			}
		})
	}
}
