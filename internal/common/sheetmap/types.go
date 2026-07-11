package sheetmap

const Version = 1

type SheetEntry struct {
	SheetIndex   int    `json:"sheet_index"`
	SheetName    string `json:"sheet_name"`
	ExportStatus string `json:"export_status"`
	PageCount    int    `json:"page_count"`
	Error        string `json:"error,omitempty"`
}

type SheetMap struct {
	Version        int          `json:"version"`
	SourceXLSX     string       `json:"source_xlsx"`
	PDFPath        string       `json:"pdf_path"`
	PageSheetNames []string     `json:"page_sheet_names"`
	SheetEntries   []SheetEntry `json:"sheet_entries"`
}
