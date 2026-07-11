package sheetmap

func ReadCompat(path string) (*SheetMap, error) {
	m, err := Read(path)
	if err != nil {
		return nil, err
	}
	if m.Version == 0 {
		m.Version = Version
	}
	return m, nil
}
