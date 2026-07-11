package exceltocsv

import (
	"bytes"
	"os"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

func ensureUTF8BOM(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if bytes.HasPrefix(data, utf8BOM) {
		return nil
	}
	withBOM := make([]byte, 0, len(utf8BOM)+len(data))
	withBOM = append(withBOM, utf8BOM...)
	withBOM = append(withBOM, data...)
	return os.WriteFile(path, withBOM, 0o644)
}
