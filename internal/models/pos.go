package models

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// POSData holds parsed POS file data (internal parsing structure)
type POSData struct {
	Headers []string `json:"headers"`
	Rows    []POSRow `json:"rows"`
}

// ParsePOS parses a KiCad POS file and returns structured data
// Supports both whitespace-delimited format (with # header) and CSV format
func ParsePOS(r io.Reader) (*POSData, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	text := string(content)

	// Check if this is CSV format (contains commas in data lines)
	if strings.Contains(text, ",") && !strings.HasPrefix(strings.TrimSpace(text), "#") {
		return parseCSVFormat(text)
	}

	// Parse KiCad whitespace-delimited format
	return parseKiCadFormat(text)
}

// parseKiCadFormat parses the KiCad POS format with # header and whitespace delimiters
func parseKiCadFormat(text string) (*POSData, error) {
	// Remove BOM if present
	text = strings.TrimPrefix(text, "\xef\xbb\xbf")

	lines := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")

	var headerLine string
	var headerLineIdx int = -1
	var dataLines []string

	// First pass: find the header line (# line containing "Ref")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		// Look for header line: starts with # and contains "Ref"
		if strings.HasPrefix(trimmed, "#") {
			content := strings.TrimPrefix(trimmed, "#")
			content = strings.TrimSpace(content)
			// Check if this line contains "Ref" (case insensitive)
			if strings.Contains(strings.ToLower(content), "ref") {
				headerLine = content
				headerLineIdx = i
				break
			}
		}
	}

	// If no # header with Ref found, try first # line
	if headerLine == "" {
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				headerLine = strings.TrimPrefix(trimmed, "#")
				headerLine = strings.TrimSpace(headerLine)
				headerLineIdx = i
				break
			}
		}
	}

	if headerLine == "" {
		return nil, fmt.Errorf("could not find KiCad POS header row (need # Ref Val ... line)")
	}

	// Second pass: collect data lines (after header, non-comment lines)
	for i, line := range lines {
		if i <= headerLineIdx {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		dataLines = append(dataLines, trimmed)
	}

	// Parse header - split by whitespace
	headers := splitByWhitespace(headerLine)
	if len(headers) == 0 {
		return nil, fmt.Errorf("empty header row")
	}

	// Build column map
	colMap := buildColumnMap(headers)

	if _, hasRef := colMap["ref"]; !hasRef {
		return nil, fmt.Errorf("header missing Ref column (found headers: %v)", headers)
	}
	if _, hasVal := colMap["val"]; !hasVal {
		return nil, fmt.Errorf("header missing Val column (found headers: %v)", headers)
	}

	data := &POSData{
		Headers: headers,
		Rows:    []POSRow{},
	}

	// Parse data rows
	for _, line := range dataLines {
		fields := splitByWhitespace(line)
		if len(fields) == 0 {
			continue
		}

		posRow := parseRowFields(fields, colMap)

		// Skip rows with no ref
		if posRow.Ref == "" {
			continue
		}

		data.Rows = append(data.Rows, posRow)
	}

	return data, nil
}

// parseCSVFormat parses CSV format POS files
func parseCSVFormat(text string) (*POSData, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")

	// Find header row
	headerIdx := -1
	var colMap map[string]int

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		fields := parseCSVLine(trimmed)
		colMap = buildColumnMap(fields)

		if _, hasRef := colMap["ref"]; hasRef {
			if _, hasVal := colMap["val"]; hasVal {
				headerIdx = i
				break
			}
		}
	}

	if headerIdx == -1 {
		return nil, fmt.Errorf("could not find KiCad POS header row (need Ref, Val columns)")
	}

	// Get headers
	headers := parseCSVLine(strings.TrimSpace(lines[headerIdx]))
	colMap = buildColumnMap(headers)

	data := &POSData{
		Headers: headers,
		Rows:    []POSRow{},
	}

	// Parse data rows
	for i := headerIdx + 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		fields := parseCSVLine(trimmed)
		if len(fields) == 0 {
			continue
		}

		posRow := parseRowFields(fields, colMap)

		if posRow.Ref == "" {
			continue
		}

		data.Rows = append(data.Rows, posRow)
	}

	return data, nil
}

// splitByWhitespace splits a line by whitespace (spaces/tabs)
func splitByWhitespace(line string) []string {
	re := regexp.MustCompile(`\s+`)
	parts := re.Split(strings.TrimSpace(line), -1)
	var result []string
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// parseCSVLine parses a CSV line
func parseCSVLine(line string) []string {
	var fields []string
	var current strings.Builder
	inQuotes := false

	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '"' {
			if inQuotes && i+1 < len(line) && line[i+1] == '"' {
				current.WriteByte('"')
				i++
			} else {
				inQuotes = !inQuotes
			}
		} else if c == ',' && !inQuotes {
			fields = append(fields, strings.TrimSpace(current.String()))
			current.Reset()
		} else {
			current.WriteByte(c)
		}
	}
	fields = append(fields, strings.TrimSpace(current.String()))
	return fields
}

// buildColumnMap creates a map of column name to index
func buildColumnMap(headers []string) map[string]int {
	colMap := make(map[string]int)
	for j, cell := range headers {
		lower := strings.ToLower(strings.TrimSpace(cell))
		if lower == "ref" || lower == "designator" {
			colMap["ref"] = j
		} else if lower == "val" || lower == "value" {
			colMap["val"] = j
		} else if lower == "package" || lower == "footprint" {
			colMap["package"] = j
		} else if lower == "posx" || lower == "mid x" || lower == "center-x(mm)" {
			colMap["posx"] = j
		} else if lower == "posy" || lower == "mid y" || lower == "center-y(mm)" {
			colMap["posy"] = j
		} else if lower == "rot" || lower == "rotation" {
			colMap["rot"] = j
		} else if lower == "side" || lower == "layer" || lower == "tb" {
			colMap["side"] = j
		}
	}
	return colMap
}

// parseRowFields extracts POSRow from fields using column map
func parseRowFields(fields []string, colMap map[string]int) POSRow {
	posRow := POSRow{}

	if idx, ok := colMap["ref"]; ok && idx < len(fields) {
		posRow.Ref = strings.TrimSpace(fields[idx])
	}
	if idx, ok := colMap["val"]; ok && idx < len(fields) {
		posRow.Val = strings.TrimSpace(fields[idx])
	}
	if idx, ok := colMap["package"]; ok && idx < len(fields) {
		posRow.Package = strings.TrimSpace(fields[idx])
	}
	if idx, ok := colMap["posx"]; ok && idx < len(fields) {
		if v, err := parseFloat(fields[idx]); err == nil {
			posRow.PosX = v
		}
	}
	if idx, ok := colMap["posy"]; ok && idx < len(fields) {
		if v, err := parseFloat(fields[idx]); err == nil {
			posRow.PosY = v
		}
	}
	if idx, ok := colMap["rot"]; ok && idx < len(fields) {
		if v, err := parseFloat(fields[idx]); err == nil {
			posRow.Rot = v
		}
	}
	if idx, ok := colMap["side"]; ok && idx < len(fields) {
		posRow.Side = strings.TrimSpace(fields[idx])
	}

	return posRow
}

// parseFloat parses a float, handling mm suffix
func parseFloat(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "mm")
	s = strings.TrimSpace(s)
	return strconv.ParseFloat(s, 64)
}

// bufio import is used implicitly by the scanner approach if needed
var _ = bufio.Scanner{}

// ConvertPOSToXFile converts parsed POS data to XFile format
func ConvertPOSToXFile(pos *POSData, filename string) *XFile {
	xf := NewXFile()
	xf.OriginalPOS = filename

	// Store original POS rows for display
	xf.POSRows = make([]POSRow, len(pos.Rows))
	copy(xf.POSRows, pos.Rows)

	// Collect unique values for Station creation
	valToStationID := make(map[string]int)
	uniqueVals := []string{}

	for _, row := range pos.Rows {
		if row.Val != "" {
			if _, exists := valToStationID[row.Val]; !exists {
				stationID := len(uniqueVals) + 1
				valToStationID[row.Val] = stationID
				uniqueVals = append(uniqueVals, row.Val)
			}
		}
	}

	// Create Stations from unique values
	for idx, val := range uniqueVals {
		station := XStation{
			No:              idx,
			ID:              idx + 1,
			DeltX:           0,
			DeltY:           0,
			FeedRates:       4,
			Note:            val,
			Height:          0.5,
			Speed:           0,
			Status:          4, // Vision enabled
			NPixSizeX:       0,
			NPixSizeY:       0,
			HeightTake:      0,
			DelayTake:       10,
			NPullStripSpeed: 85,
			NThreshold:      110,
			NVisualRadio:    200,
			Select:          false,
			PHead:           1,
			DNP:             false,
		}
		xf.Stations = append(xf.Stations, station)
	}

	// Create Components from POS rows
	for idx, row := range pos.Rows {
		stNo := 1
		if id, ok := valToStationID[row.Val]; ok {
			stNo = id
		}

		note := ""
		if row.Ref != "" && row.Package != "" {
			note = row.Ref + " - " + row.Package
		} else if row.Ref != "" {
			note = row.Ref
		} else if row.Package != "" {
			note = row.Package
		}

		comp := XComponent{
			No:      idx,
			ID:      idx + 1,
			PHead:   1,
			STNo:    stNo,
			DeltX:   row.PosX,
			DeltY:   row.PosY,
			Angle:   row.Rot,
			Height:  0.5,
			Skip:    4, // Match Station Status=4 (vision enabled)
			Speed:   0,
			Explain: row.Val,
			Note:    note,
			Delay:   0,
			Select:  false,
			DNP:     false,
		}
		xf.Components = append(xf.Components, comp)
	}

	return xf
}
