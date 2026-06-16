package writer

import "crypto/md5"

// ComputeMD5Footer computes the MD5 hash over the given data, which represents
// all preceding bytes of a DSF file (cookie + version + all atoms). It returns
// the 16-byte hash that serves as the DSF file footer.
func ComputeMD5Footer(data []byte) [16]byte {
	return md5.Sum(data)
}

// AppendMD5Footer appends the 16-byte MD5 footer to the given data and returns
// the complete DSF file contents. The hash is computed over all bytes in data
// (cookie + version + all atoms) and the resulting 16-byte digest is appended.
func AppendMD5Footer(data []byte) []byte {
	hash := ComputeMD5Footer(data)
	return append(data, hash[:]...)
}
