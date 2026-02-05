package models

import (
	"fmt"
	"strings"
	"time"
)

// DPVValidationError represents a validation error
type DPVValidationError struct {
	Type    string `json:"type"`
	Field   string `json:"field,omitempty"`
	Row     int    `json:"row,omitempty"`
	Message string `json:"message"`
}

// DPVValidationResult contains validation results
type DPVValidationResult struct {
	Valid    bool                 `json:"valid"`
	Errors   []DPVValidationError `json:"errors"`
	Warnings []DPVValidationError `json:"warnings"`
}

// ValidateDPV performs comprehensive validation per DPVFileFormat.txt specification
func ValidateDPV(xf *XFile, filename string) *DPVValidationResult {
	result := &DPVValidationResult{
		Valid:    true,
		Errors:   []DPVValidationError{},
		Warnings: []DPVValidationError{},
	}

	// Filter out DNP items for validation
	activeComponents := []XComponent{}
	activeStations := []XStation{}

	for _, c := range xf.Components {
		if !c.DNP {
			activeComponents = append(activeComponents, c)
		}
	}
	for _, s := range xf.Stations {
		if !s.DNP {
			activeStations = append(activeStations, s)
		}
	}

	// === STATION TABLE VALIDATION ===

	// Check Station IDs are unique and within valid range
	stationIDs := make(map[int]bool)
	for i, s := range activeStations {
		if stationIDs[s.ID] {
			result.Errors = append(result.Errors, DPVValidationError{
				Type:    "duplicate_station_id",
				Field:   "Station.ID",
				Row:     i,
				Message: fmt.Sprintf("Duplicate Station ID %d at row %d", s.ID, i),
			})
			result.Valid = false
		}
		stationIDs[s.ID] = true

		// Station IDs >= 100 are reserved for machine configuration and will cause head crashes
		if s.ID >= 100 {
			result.Errors = append(result.Errors, DPVValidationError{
				Type:    "reserved_station_id",
				Field:   "Station.ID",
				Row:     i,
				Message: fmt.Sprintf("Station ID %d is reserved (IDs >= 100 are machine-reserved and will cause head crashes)", s.ID),
			})
			result.Valid = false
		}

		// Check for IDs in undefined ranges (30-35, 65-70)
		if (s.ID >= 30 && s.ID <= 35) || (s.ID >= 65 && s.ID <= 70) {
			result.Warnings = append(result.Warnings, DPVValidationError{
				Type:    "undefined_station_id",
				Field:   "Station.ID",
				Row:     i,
				Message: fmt.Sprintf("Station ID %d is in an undefined range (valid: 1-29 left reels, 36-64 right reels, 71-84 front tray, 85-90 vibratory, 91-99 IC trays)", s.ID),
			})
		}
	}

	// Check Station No. is sequential (0 to N-1)
	for i, s := range activeStations {
		if s.No != i {
			result.Warnings = append(result.Warnings, DPVValidationError{
				Type:    "station_no_sequence",
				Field:   "Station.No.",
				Row:     i,
				Message: fmt.Sprintf("Station No. %d should be %d (will be renumbered on export)", s.No, i),
			})
		}
	}

	// Check Station Status flags
	for i, s := range activeStations {
		if s.Status < 0 || s.Status > 15 {
			result.Errors = append(result.Errors, DPVValidationError{
				Type:    "invalid_station_status",
				Field:   "Station.Status",
				Row:     i,
				Message: fmt.Sprintf("Station Status %d is invalid (must be 0-15)", s.Status),
			})
			result.Valid = false
		}
	}

	// Check Station FeedRates
	for i, s := range activeStations {
		if s.FeedRates != 2 && s.FeedRates != 4 && s.FeedRates != 8 {
			result.Warnings = append(result.Warnings, DPVValidationError{
				Type:    "unusual_feedrate",
				Field:   "Station.FeedRates",
				Row:     i,
				Message: fmt.Sprintf("Station FeedRates %d is unusual (typically 2, 4, or 8)", s.FeedRates),
			})
		}
	}

	// Check Station Speed (must be 0 or >= 50, where 0 means 100%)
	for i, s := range activeStations {
		if s.Speed != 0 && s.Speed < 50 {
			result.Errors = append(result.Errors, DPVValidationError{
				Type:    "invalid_station_speed",
				Field:   "Station.Speed",
				Row:     i,
				Message: fmt.Sprintf("Station Speed %d is invalid (must be 0 for 100%%, or 50-100)", s.Speed),
			})
			result.Valid = false
		}
	}

	// Check Station PHead (must be 1 or 2)
	for i, s := range activeStations {
		if s.PHead != 1 && s.PHead != 2 {
			result.Errors = append(result.Errors, DPVValidationError{
				Type:    "invalid_station_phead",
				Field:   "Station.PHead",
				Row:     i,
				Message: fmt.Sprintf("Station PHead %d must be 1 (left nozzle) or 2 (right nozzle)", s.PHead),
			})
			result.Valid = false
		}
	}

	// Check Station nThreshold (must be 0 or 1-256)
	for i, s := range activeStations {
		if s.NThreshold != 0 && (s.NThreshold < 1 || s.NThreshold > 256) {
			result.Errors = append(result.Errors, DPVValidationError{
				Type:    "invalid_threshold",
				Field:   "Station.nThreshold",
				Row:     i,
				Message: fmt.Sprintf("Station nThreshold %d is invalid (must be 0 for default, or 1-256)", s.NThreshold),
			})
			result.Valid = false
		}
	}

	// Check Station Height (max 5mm per spec)
	for i, s := range activeStations {
		if s.Height > 5.0 {
			result.Errors = append(result.Errors, DPVValidationError{
				Type:    "station_height_exceeded",
				Field:   "Station.Height",
				Row:     i,
				Message: fmt.Sprintf("Station Height %.2f exceeds maximum 5mm", s.Height),
			})
			result.Valid = false
		}
		if s.Height < 0 {
			result.Errors = append(result.Errors, DPVValidationError{
				Type:    "station_height_negative",
				Field:   "Station.Height",
				Row:     i,
				Message: fmt.Sprintf("Station Height %.2f cannot be negative", s.Height),
			})
			result.Valid = false
		}
	}

	// Check if all Station coordinates are zero (need calibration)
	allStationCoordsZero := true
	for _, s := range activeStations {
		if s.DeltX != 0 || s.DeltY != 0 {
			allStationCoordsZero = false
			break
		}
	}
	if allStationCoordsZero && len(activeStations) > 0 {
		result.Warnings = append(result.Warnings, DPVValidationError{
			Type:    "stations_need_calibration",
			Field:   "Station.DeltX/DeltY",
			Message: "All Material Stack coordinates are zero. You will need to calibrate feeder positions on the machine before running.",
		})
	}

	// === COMPONENT TABLE VALIDATION ===

	// Check Component No. is sequential (0 to N-1)
	for i, c := range activeComponents {
		if c.No != i {
			result.Warnings = append(result.Warnings, DPVValidationError{
				Type:    "component_no_sequence",
				Field:   "EComponent.No.",
				Row:     i,
				Message: fmt.Sprintf("Component No. %d should be %d (will be renumbered on export)", c.No, i),
			})
		}
	}

	// Check Component PHead (must be 1 or 2)
	for i, c := range activeComponents {
		if c.PHead != 1 && c.PHead != 2 {
			result.Errors = append(result.Errors, DPVValidationError{
				Type:    "invalid_phead",
				Field:   "EComponent.PHead",
				Row:     i,
				Message: fmt.Sprintf("Component PHead %d must be 1 or 2", c.PHead),
			})
			result.Valid = false
		}
	}

	// Check Component STNo. references valid Station ID
	for i, c := range activeComponents {
		if !stationIDs[c.STNo] {
			result.Errors = append(result.Errors, DPVValidationError{
				Type:    "orphan_component",
				Field:   "EComponent.STNo.",
				Row:     i,
				Message: fmt.Sprintf("Component STNo. %d references non-existent Station ID", c.STNo),
			})
			result.Valid = false
		}
	}

	// Check Component Skip matches Station Status for vision flag
	// Skip/Status mismatches will be auto-resolved on export, just warn here
	stationStatusMap := make(map[int]int)
	for _, s := range activeStations {
		stationStatusMap[s.ID] = s.Status
	}

	for i, c := range activeComponents {
		stationStatus, ok := stationStatusMap[c.STNo]
		if !ok {
			continue // Already reported as orphan
		}

		// Check vision flag consistency - warn if mismatch (will be auto-fixed on export)
		compHasVision := (c.Skip & 4) != 0
		stationHasVision := (stationStatus & 4) != 0

		if stationHasVision && !compHasVision {
			result.Warnings = append(result.Warnings, DPVValidationError{
				Type:    "skip_status_mismatch",
				Field:   "EComponent.Skip",
				Row:     i,
				Message: fmt.Sprintf("Component Skip=%d will be updated to include vision flag from Station %d (Status=%d)", c.Skip, c.STNo, stationStatus),
			})
		}
	}

	// Check Component coordinates are positive
	for i, c := range activeComponents {
		if c.DeltX < 0 || c.DeltY < 0 {
			result.Warnings = append(result.Warnings, DPVValidationError{
				Type:    "negative_coordinates",
				Field:   "EComponent.DeltX/DeltY",
				Row:     i,
				Message: fmt.Sprintf("Component has negative coordinates (%.2f, %.2f) - all positions should be positive", c.DeltX, c.DeltY),
			})
		}
	}

	// Check Component Angle is in valid range (-180 to 180)
	for i, c := range activeComponents {
		if c.Angle < -180 || c.Angle > 180 {
			result.Warnings = append(result.Warnings, DPVValidationError{
				Type:    "angle_out_of_range",
				Field:   "EComponent.Angle",
				Row:     i,
				Message: fmt.Sprintf("Component Angle %.2f should be between -180 and 180", c.Angle),
			})
		}
	}

	// Check Component Speed (must be 0 or >= 50, where 0 means 100%)
	for i, c := range activeComponents {
		if c.Speed != 0 && c.Speed < 50 {
			result.Errors = append(result.Errors, DPVValidationError{
				Type:    "invalid_component_speed",
				Field:   "EComponent.Speed",
				Row:     i,
				Message: fmt.Sprintf("Component Speed %d is invalid (must be 0 for 100%%, or 50-100)", c.Speed),
			})
			result.Valid = false
		}
	}

	// Machine bug: Need at least 2 EComponent rows for 3-point calibration to work
	if len(activeComponents) == 1 {
		result.Warnings = append(result.Warnings, DPVValidationError{
			Type:    "single_component",
			Field:   "EComponent",
			Message: "Only 1 component defined - machine requires at least 2 components for LR fiducial calibration to work (known bug)",
		})
	}

	// Check Component Height matches Station Height
	for i, c := range activeComponents {
		for _, s := range activeStations {
			if s.ID == c.STNo && c.Height != s.Height {
				result.Warnings = append(result.Warnings, DPVValidationError{
					Type:    "height_mismatch",
					Field:   "EComponent.Height",
					Row:     i,
					Message: fmt.Sprintf("Component Height %.2f differs from Station %d Height %.2f", c.Height, s.ID, s.Height),
				})
				break
			}
		}
	}

	// === PCB SIZE VALIDATION (CHM-T48VB specs) ===
	// Machine specs: PCB max size 345mm(L) x 355mm(W), XY travel 510mm x 460mm
	const maxPCBX = 345.0
	const maxPCBY = 355.0

	var maxX, maxY float64
	for _, c := range activeComponents {
		// Apply global offset to get actual placement position
		x := c.DeltX + xf.GlobalOffset.X
		y := c.DeltY + xf.GlobalOffset.Y
		if x > maxX {
			maxX = x
		}
		if y > maxY {
			maxY = y
		}
	}

	if maxX > maxPCBX {
		result.Warnings = append(result.Warnings, DPVValidationError{
			Type:    "pcb_size_x",
			Field:   "EComponent.DeltX",
			Message: fmt.Sprintf("Component X position %.2fmm exceeds PCB max width of %.0fmm (CHM-T48VB limit)", maxX, maxPCBX),
		})
	}
	if maxY > maxPCBY {
		result.Warnings = append(result.Warnings, DPVValidationError{
			Type:    "pcb_size_y",
			Field:   "EComponent.DeltY",
			Message: fmt.Sprintf("Component Y position %.2fmm exceeds PCB max length of %.0fmm (CHM-T48VB limit)", maxY, maxPCBY),
		})
	}

	// === PANEL_ARRAY VALIDATION ===
	// Panel_Array is REQUIRED - machine won't allow PCB calibration without it
	if len(xf.PanelArray) == 0 {
		result.Errors = append(result.Errors, DPVValidationError{
			Type:    "missing_panel_array",
			Field:   "Panel_Array",
			Message: "Panel_Array table is required - machine won't allow PCB calibration without it",
		})
		result.Valid = false
	} else {
		pa := xf.PanelArray[0]
		if pa.NumX < 1 || pa.NumY < 1 {
			result.Errors = append(result.Errors, DPVValidationError{
				Type:    "invalid_panel_array",
				Field:   "Panel_Array.NumX/NumY",
				Row:     0,
				Message: fmt.Sprintf("Panel_Array NumX (%d) and NumY (%d) must be at least 1", pa.NumX, pa.NumY),
			})
			result.Valid = false
		}
	}

	// === FILE HEADER VALIDATION ===
	if filename == "" {
		result.Errors = append(result.Errors, DPVValidationError{
			Type:    "missing_filename",
			Field:   "FILE",
			Message: "Output filename is required",
		})
		result.Valid = false
	} else if !strings.HasSuffix(strings.ToLower(filename), ".dpv") {
		result.Warnings = append(result.Warnings, DPVValidationError{
			Type:    "filename_extension",
			Field:   "FILE",
			Message: fmt.Sprintf("Filename '%s' should have .dpv extension", filename),
		})
	}

	return result
}

// GenerateDPV generates DPV file content from XFile
// This excludes DNP rows and applies global offset
func GenerateDPV(xf *XFile, filename string) (string, error) {
	var sb strings.Builder

	// Validate first
	validation := ValidateDPV(xf, filename)
	if !validation.Valid {
		errMsgs := []string{}
		for _, e := range validation.Errors {
			errMsgs = append(errMsgs, e.Message)
		}
		return "", fmt.Errorf("DPV validation failed:\n%s", strings.Join(errMsgs, "\n"))
	}

	// Filter out DNP items
	activeComponents := []XComponent{}
	activeStations := []XStation{}
	usedStationIDs := make(map[int]bool)

	for _, c := range xf.Components {
		if !c.DNP {
			activeComponents = append(activeComponents, c)
			usedStationIDs[c.STNo] = true
		}
	}
	for _, s := range xf.Stations {
		if !s.DNP && usedStationIDs[s.ID] {
			activeStations = append(activeStations, s)
		}
	}

	// Header
	now := time.Now()
	sb.WriteString("separated\r\n")
	sb.WriteString(fmt.Sprintf("FILE,%s\r\n", filename))
	sb.WriteString(fmt.Sprintf("PCBFILE,%s\r\n", xf.OriginalPOS))
	sb.WriteString(fmt.Sprintf("DATE,%d/%02d/%02d\r\n", now.Year(), now.Month(), now.Day()))
	sb.WriteString(fmt.Sprintf("TIME,%02d:%02d:%02d\r\n", now.Hour(), now.Minute(), now.Second()))
	sb.WriteString("PANELYPE,1\r\n")

	// Station table (V1 format without custom PHead column)
	sb.WriteString("\r\n")
	sb.WriteString("Table,No.,ID,DeltX,DeltY,FeedRates,Note,Height,Speed,Status,nPixSizeX,nPixSizeY,HeightTake,DelayTake,nPullStripSpeed,nThreshold,nVisualRadio\r\n")
	for i, s := range activeStations {
		sb.WriteString(fmt.Sprintf("Station,%d,%d,%.2f,%.2f,%d,%s,%.2f,%d,%d,%d,%d,%.2f,%d,%d,%d,%d\r\n",
			i, s.ID, s.DeltX, s.DeltY, s.FeedRates, csvEscape(s.Note),
			s.Height, s.Speed, s.Status, s.NPixSizeX, s.NPixSizeY,
			s.HeightTake, s.DelayTake, s.NPullStripSpeed, s.NThreshold, s.NVisualRadio))
	}

	// Panel_Array table
	sb.WriteString("\r\n")
	sb.WriteString("Table,No.,ID,IntervalX,IntervalY,NumX,NumY\r\n")
	for i, pa := range xf.PanelArray {
		sb.WriteString(fmt.Sprintf("Panel_Array,%d,%d,%.2f,%.2f,%d,%d\r\n",
			i, pa.ID, pa.IntervalX, pa.IntervalY, pa.NumX, pa.NumY))
	}

	// Panel_Coord table
	sb.WriteString("\r\n")
	sb.WriteString("Table,No.,ID,DeltX,DeltY\r\n")
	for i, pc := range xf.PanelCoord {
		sb.WriteString(fmt.Sprintf("Panel_Coord,%d,%d,%.2f,%.2f\r\n",
			i, pc.ID, pc.DeltX, pc.DeltY))
	}

	// Build Station Status map for auto-fixing Skip values
	stationStatusMap := make(map[int]int)
	for _, s := range activeStations {
		stationStatusMap[s.ID] = s.Status
	}

	// EComponent table (with PHead in position 3)
	sb.WriteString("\r\n")
	sb.WriteString("Table,No.,ID,PHead,STNo.,DeltX,DeltY,Angle,Height,Skip,Speed,Explain,Note,Delay\r\n")
	for i, c := range activeComponents {
		// Apply global offset
		deltX := c.DeltX + xf.GlobalOffset.X
		deltY := c.DeltY + xf.GlobalOffset.Y

		// Auto-fix Skip to match Station Status flags (vision, vacuum, etc.)
		skip := c.Skip
		if stationStatus, ok := stationStatusMap[c.STNo]; ok {
			// Ensure component Skip includes station's vision flag (bit 2 = 4)
			if (stationStatus&4) != 0 && (skip&4) == 0 {
				skip |= 4
			}
			// Ensure component Skip includes station's vacuum flag (bit 1 = 2)
			if (stationStatus&2) != 0 && (skip&2) == 0 {
				skip |= 2
			}
		}

		sb.WriteString(fmt.Sprintf("EComponent,%d,%d,%d,%d,%.2f,%.2f,%.2f,%.2f,%d,%d,%s,%s,%d\r\n",
			i, c.ID, c.PHead, c.STNo, deltX, deltY, c.Angle,
			c.Height, skip, c.Speed, csvEscape(c.Explain), csvEscape(c.Note), c.Delay))
	}

	// ICTray table (empty, header only)
	sb.WriteString("\r\n")
	sb.WriteString("Table,No.,ID,CenterX,CenterY,IntervalX,IntervalY,NumX,NumY,Start\r\n")

	// PcbCalib table
	sb.WriteString("\r\n")
	sb.WriteString("Table,No.,nType,nAlg,nFinished\r\n")
	sb.WriteString("PcbCalib,0,0,0,0\r\n")

	// CalibPoint table (3 calibration points: UL, LR, LL)
	sb.WriteString("\r\n")
	sb.WriteString("Table,No.,ID,offsetX,offsetY,Note,Model,Type,DevX,DevY\r\n")
	sb.WriteString("CalibPoint,0,1,0,0,,0,0,0,0\r\n")
	sb.WriteString("CalibPoint,1,2,0,0,,0,0,0,0\r\n")
	sb.WriteString("CalibPoint,2,3,0,0,,0,0,0,0\r\n")

	// CalibFator table
	sb.WriteString("\r\n")
	sb.WriteString("Table,No.,PCBX1,PCBY1,PCBX2,PCBY2,PCBX3,PCBY3,SMTX1,SMTY1,SMTX2,SMTY2,SMTX3,SMTY3,DeltaAngle\r\n")
	sb.WriteString("CalibFator,0,0,0,0,0,0,0,0,0,0,0,0,0,0\r\n")

	return sb.String(), nil
}

// csvEscape escapes a string for CSV output
func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\r\n") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

// GenerateReadme creates a README.txt with setup instructions for the export package
func GenerateReadme(xf *XFile, filename string) string {
	var sb strings.Builder

	sb.WriteString("CharmTool Export Package - Setup Checklist\r\n")
	sb.WriteString("==========================================\r\n")
	sb.WriteString(fmt.Sprintf("File: %s\r\n", filename))
	sb.WriteString(fmt.Sprintf("Generated: %s\r\n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString("\r\n")

	sb.WriteString("BEFORE RUNNING THIS JOB ON THE MACHINE:\r\n")
	sb.WriteString("---------------------------------------\r\n")
	sb.WriteString("\r\n")

	sb.WriteString("1. IMPORT THE DPV FILE\r\n")
	sb.WriteString("   File > Open > Select the .dpv file\r\n")
	sb.WriteString("\r\n")

	sb.WriteString("2. SET PCB COORDINATES\r\n")
	sb.WriteString("   Run > Edit > PCB Calibrate\r\n")
	sb.WriteString("   - Set the board origin (0,0) position\r\n")
	sb.WriteString("\r\n")

	sb.WriteString("3. SET THREE CALIBRATION POINTS\r\n")
	sb.WriteString("   Run > Edit > PCB Calibrate\r\n")
	sb.WriteString("   - Calibrate UL (Upper Left), LR (Lower Right), LL (Lower Left)\r\n")
	sb.WriteString("   - Use component positions for better accuracy\r\n")
	sb.WriteString("\r\n")

	// Check if Material Stacks need calibration
	allCoordsZero := true
	for _, s := range xf.Stations {
		if !s.DNP && (s.DeltX != 0 || s.DeltY != 0) {
			allCoordsZero = false
			break
		}
	}

	if allCoordsZero {
		sb.WriteString("4. *** MATERIAL STACKS NEED CALIBRATION ***\r\n")
		sb.WriteString("   Run > Edit > MStack > Edit > Coordinate Set\r\n")
		sb.WriteString("   - All feeder pocket positions are currently at 0,0\r\n")
		sb.WriteString("   - Calibrate each Station's DeltX/DeltY for accurate pickup\r\n")
		sb.WriteString("   - Or import a previously calibrated .stack file\r\n")
		sb.WriteString("\r\n")
	} else {
		sb.WriteString("4. VERIFY MATERIAL STACK POSITIONS\r\n")
		sb.WriteString("   Run > Edit > MStack\r\n")
		sb.WriteString("   - Material Stack coordinates are set\r\n")
		sb.WriteString("   - Verify positions if feeders have been moved\r\n")
		sb.WriteString("\r\n")
	}

	sb.WriteString("5. VERIFY COMPONENT ASSIGNMENTS\r\n")
	sb.WriteString("   Run > Edit > Batch Edit\r\n")
	sb.WriteString("   - Check that components are assigned to correct feeders\r\n")
	sb.WriteString("   - Remove any invalid entries\r\n")
	sb.WriteString("\r\n")

	sb.WriteString("6. RUN A DRY TEST\r\n")
	sb.WriteString("   - Run without vacuum to verify positions\r\n")
	sb.WriteString("   - Check nozzle movements over feeders and board\r\n")
	sb.WriteString("\r\n")

	sb.WriteString("PACKAGE CONTENTS:\r\n")
	sb.WriteString("-----------------\r\n")
	baseName := filename
	if idx := len(baseName) - 4; idx > 0 && baseName[idx:] == ".dpv" {
		baseName = baseName[:idx]
	}
	sb.WriteString(fmt.Sprintf("- %s.dpv      : DPV file for the machine\r\n", baseName))
	sb.WriteString(fmt.Sprintf("- %s.stack    : Material Stack backup (with PHead)\r\n", baseName))
	sb.WriteString(fmt.Sprintf("- %s.pos      : Original POS file\r\n", baseName))
	sb.WriteString(fmt.Sprintf("- %s.log      : Session log\r\n", baseName))
	sb.WriteString("- material.stacks : Calibrated feeder positions (reusable)\r\n")
	sb.WriteString("- README.txt      : This file\r\n")
	sb.WriteString("\r\n")
	sb.WriteString("TIP: Import material.stacks into future projects to reuse\r\n")
	sb.WriteString("     your calibrated feeder positions.\r\n")
	sb.WriteString("\r\n")

	sb.WriteString("SUMMARY:\r\n")
	sb.WriteString("--------\r\n")
	activeComps := 0
	activeStations := 0
	for _, c := range xf.Components {
		if !c.DNP {
			activeComps++
		}
	}
	for _, s := range xf.Stations {
		if !s.DNP {
			activeStations++
		}
	}
	sb.WriteString(fmt.Sprintf("Components to place: %d\r\n", activeComps))
	sb.WriteString(fmt.Sprintf("Material Stacks: %d\r\n", activeStations))
	sb.WriteString("\r\n")

	sb.WriteString("Generated by CharmTool - https://github.com/rickmcneely/CharmTools\r\n")

	return sb.String()
}
