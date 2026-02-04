package models

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ParseStack parses a STACK file and returns Station data
// STACK files are DPV-like files containing only Station table data
func ParseStack(r io.Reader) ([]XStation, error) {
	content, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read stack file: %w", err)
	}

	text := string(content)
	lines := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")

	// Check for DPV format markers
	isDPV := strings.Contains(strings.ToLower(text), "separated") ||
		strings.Contains(strings.ToLower(text), "station")

	if !isDPV {
		return nil, fmt.Errorf("not a valid STACK file (missing 'separated' or 'Station' markers)")
	}

	// Parse as DPV format
	var header []string
	var stations []XStation

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip header lines
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "separated") ||
			strings.HasPrefix(lower, "file,") ||
			strings.HasPrefix(lower, "pcbfile,") ||
			strings.HasPrefix(lower, "date,") ||
			strings.HasPrefix(lower, "time,") ||
			strings.HasPrefix(lower, "panelype,") {
			continue
		}

		// Parse CSV row
		reader := csv.NewReader(strings.NewReader(line))
		reader.FieldsPerRecord = -1
		rows, err := reader.ReadAll()
		if err != nil || len(rows) == 0 {
			continue
		}
		row := rows[0]
		if len(row) == 0 {
			continue
		}

		first := strings.TrimSpace(row[0])

		// Table header
		if first == "Table" {
			header = row
			continue
		}

		// Station data row
		if first == "Station" {
			if len(header) == 0 {
				// Default header if none found
				header = []string{"Table", "No.", "ID", "DeltX", "DeltY", "FeedRates", "Note",
					"Height", "Speed", "Status", "nPixSizeX", "nPixSizeY", "HeightTake",
					"DelayTake", "nPullStripSpeed", "nThreshold", "nVisualRadio"}
			}

			station := parseStationRow(header, row)
			stations = append(stations, station)
		}
	}

	if len(stations) == 0 {
		return nil, fmt.Errorf("no Station data found in stack file")
	}

	return stations, nil
}

// parseStationRow parses a single Station row using the header for column mapping
func parseStationRow(header, row []string) XStation {
	s := XStation{
		FeedRates:       4,
		Height:          0.5,
		Status:          4,
		DelayTake:       10,
		NPullStripSpeed: 85,
		NThreshold:      110,
		NVisualRadio:    200,
		PHead:           1,
	}

	colMap := make(map[string]int)
	for i, h := range header {
		colMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	getValue := func(name string) string {
		if idx, ok := colMap[name]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	getInt := func(name string, def int) int {
		v := getValue(name)
		if v == "" {
			return def
		}
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
		return def
	}

	getFloat := func(name string, def float64) float64 {
		v := getValue(name)
		if v == "" {
			return def
		}
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
		return def
	}

	s.No = getInt("no.", 0)
	s.ID = getInt("id", 1)
	s.DeltX = getFloat("deltx", 0)
	s.DeltY = getFloat("delty", 0)
	s.FeedRates = getInt("feedrates", 4)
	s.Note = getValue("note")
	s.Height = getFloat("height", 0.5)
	s.Speed = getInt("speed", 0)
	s.Status = getInt("status", 4)

	// Support both V0 (SizeX/SizeY) and V1 (nPixSizeX/nPixSizeY) formats
	if v := getInt("npixsizex", -1); v >= 0 {
		s.NPixSizeX = v
	} else {
		s.NPixSizeX = getInt("sizex", 0)
	}
	if v := getInt("npixsizey", -1); v >= 0 {
		s.NPixSizeY = v
	} else {
		s.NPixSizeY = getInt("sizey", 0)
	}

	s.HeightTake = getFloat("heighttake", 0)
	s.DelayTake = getInt("delaytake", 10)
	s.NPullStripSpeed = getInt("npullstripspeed", 85)
	s.NThreshold = getInt("nthreshold", 110)
	s.NVisualRadio = getInt("nvisualradio", 200)

	// Extended field: PHead (if present in custom stack format, default to 1)
	s.PHead = getInt("phead", 1)

	return s
}

// MergeStationsIntoXFile merges station data into an XFile
// Matching is done by Note field (component value)
// If a station Note matches an existing station, it updates that station
// Otherwise, the station is added
func MergeStationsIntoXFile(xf *XFile, stations []XStation, filename string) int {
	merged := 0

	// Create map of existing stations by Note
	noteToIdx := make(map[string]int)
	for i, s := range xf.Stations {
		if s.Note != "" {
			noteToIdx[s.Note] = i
		}
	}

	// Track which incoming stations matched
	for _, incoming := range stations {
		if idx, ok := noteToIdx[incoming.Note]; ok {
			// Update existing station (preserve ID to maintain component links)
			existingID := xf.Stations[idx].ID
			xf.Stations[idx] = incoming
			xf.Stations[idx].ID = existingID
			merged++
		} else {
			// Add new station with next available ID
			maxID := 0
			for _, s := range xf.Stations {
				if s.ID > maxID {
					maxID = s.ID
				}
			}
			incoming.ID = maxID + 1
			incoming.No = len(xf.Stations)
			xf.Stations = append(xf.Stations, incoming)
		}
	}

	// Add filename to loaded stacks list
	if filename != "" {
		found := false
		for _, f := range xf.StackFiles {
			if f == filename {
				found = true
				break
			}
		}
		if !found {
			xf.StackFiles = append(xf.StackFiles, filename)
		}
	}

	// Re-derive component STNo. based on updated Station Notes
	rederiveComponentSTNo(xf)

	return merged
}

// rederiveComponentSTNo updates component STNo. to match Station ID by Note
func rederiveComponentSTNo(xf *XFile) {
	// Build Note -> ID map
	noteToID := make(map[string]int)
	for _, s := range xf.Stations {
		if s.Note != "" {
			noteToID[s.Note] = s.ID
		}
	}

	// Update component STNo. based on Explain (Val) matching Station Note
	for i := range xf.Components {
		if id, ok := noteToID[xf.Components[i].Explain]; ok {
			xf.Components[i].STNo = id
		}
	}
}

// GenerateStack generates a STACK file from XFile stations (for DPV export)
func GenerateStack(xf *XFile) string {
	var sb strings.Builder

	sb.WriteString("separated\r\n")
	sb.WriteString("FILE,MaterialStack.stack\r\n")
	sb.WriteString("PANELYPE,1\r\n")
	sb.WriteString("\r\n")

	// Include PHead column in stack format
	sb.WriteString("Table,No.,ID,PHead,DeltX,DeltY,FeedRates,Note,Height,Speed,Status,nPixSizeX,nPixSizeY,HeightTake,DelayTake,nPullStripSpeed,nThreshold,nVisualRadio\r\n")

	for i, s := range xf.Stations {
		if s.DNP {
			continue
		}
		sb.WriteString(fmt.Sprintf("Station,%d,%d,%d,%.2f,%.2f,%d,%s,%.2f,%d,%d,%d,%d,%.2f,%d,%d,%d,%d\r\n",
			i, s.ID, s.PHead, s.DeltX, s.DeltY, s.FeedRates, stackCsvEscape(s.Note),
			s.Height, s.Speed, s.Status, s.NPixSizeX, s.NPixSizeY,
			s.HeightTake, s.DelayTake, s.NPullStripSpeed, s.NThreshold, s.NVisualRadio))
	}

	return sb.String()
}

// GenerateStacksFile generates a .stacks file from XFile stations (for Material Stacks export)
func GenerateStacksFile(xf *XFile) string {
	var sb strings.Builder

	sb.WriteString("separated\r\n")
	sb.WriteString("FILE,material.stacks\r\n")
	sb.WriteString("PANELYPE,1\r\n")
	sb.WriteString("\r\n")

	// Include PHead column in stacks format
	sb.WriteString("Table,No.,ID,PHead,DeltX,DeltY,FeedRates,Note,Height,Speed,Status,nPixSizeX,nPixSizeY,HeightTake,DelayTake,nPullStripSpeed,nThreshold,nVisualRadio\r\n")

	idx := 0
	for _, s := range xf.Stations {
		if s.DNP {
			continue
		}
		sb.WriteString(fmt.Sprintf("Station,%d,%d,%d,%.2f,%.2f,%d,%s,%.2f,%d,%d,%d,%d,%.2f,%d,%d,%d,%d\r\n",
			idx, s.ID, s.PHead, s.DeltX, s.DeltY, s.FeedRates, stackCsvEscape(s.Note),
			s.Height, s.Speed, s.Status, s.NPixSizeX, s.NPixSizeY,
			s.HeightTake, s.DelayTake, s.NPullStripSpeed, s.NThreshold, s.NVisualRadio))
		idx++
	}

	return sb.String()
}

// MergeStacksFile parses a .stacks file and merges into XFile
// Returns (merged count, added count, error)
func MergeStacksFile(xf *XFile, content string) (int, int, error) {
	stations, err := ParseStack(strings.NewReader(content))
	if err != nil {
		return 0, 0, err
	}

	merged := 0
	added := 0

	// Create map of existing stations by Note
	noteToIdx := make(map[string]int)
	for i, s := range xf.Stations {
		if s.Note != "" {
			noteToIdx[s.Note] = i
		}
	}

	// Merge incoming stations
	for _, incoming := range stations {
		if idx, ok := noteToIdx[incoming.Note]; ok {
			// Update existing station (preserve ID to maintain component links)
			existingID := xf.Stations[idx].ID
			existingNo := xf.Stations[idx].No
			xf.Stations[idx] = incoming
			xf.Stations[idx].ID = existingID
			xf.Stations[idx].No = existingNo
			merged++
		} else {
			// Add new station with next available ID
			maxID := 0
			for _, s := range xf.Stations {
				if s.ID > maxID {
					maxID = s.ID
				}
			}
			incoming.ID = maxID + 1
			incoming.No = len(xf.Stations)
			xf.Stations = append(xf.Stations, incoming)
			added++
		}
	}

	// Re-derive component STNo. based on updated Station Notes
	rederiveComponentSTNo(xf)

	return merged, added, nil
}

// stackCsvEscape escapes a string for CSV output
func stackCsvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\r\n") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}
