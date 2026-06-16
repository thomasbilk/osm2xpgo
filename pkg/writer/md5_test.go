package writer

import (
	"crypto/md5"
	"testing"
)

func TestComputeMD5Footer(t *testing.T) {
	data := []byte("XPLNEDSF\x01\x00\x00\x00some atom payload")
	want := md5.Sum(data)
	got := ComputeMD5Footer(data)
	if got != want {
		t.Errorf("ComputeMD5Footer mismatch: got %x, want %x", got, want)
	}
}

func TestAppendMD5Footer(t *testing.T) {
	data := []byte("XPLNEDSF\x01\x00\x00\x00some atom payload")
	result := AppendMD5Footer(data)

	// Result should be 16 bytes longer than original data.
	if len(result) != len(data)+16 {
		t.Fatalf("expected length %d, got %d", len(data)+16, len(result))
	}

	// The first part should be the original data unchanged.
	for i := range data {
		if result[i] != data[i] {
			t.Fatalf("data at byte %d was modified: got %x, want %x", i, result[i], data[i])
		}
	}

	// The trailing 16 bytes should be the MD5 of the original data.
	want := md5.Sum(data)
	var got [16]byte
	copy(got[:], result[len(data):])
	if got != want {
		t.Errorf("footer hash mismatch: got %x, want %x", got, want)
	}
}

func TestAppendMD5FooterEmpty(t *testing.T) {
	data := []byte{}
	result := AppendMD5Footer(data)

	if len(result) != 16 {
		t.Fatalf("expected length 16 for empty input, got %d", len(result))
	}

	// MD5 of empty input is a known constant.
	want := md5.Sum([]byte{})
	var got [16]byte
	copy(got[:], result)
	if got != want {
		t.Errorf("footer hash of empty data mismatch: got %x, want %x", got, want)
	}
}
