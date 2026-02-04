# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

CharmTool Web converts KiCad POS placement files to Charmhigh CHMT-48VB pick-and-place machine DPV format. It's a Go backend with a single-page HTML/JS frontend.

## Build and Run

```bash
# Build
go build -o charmtool ./cmd/server

# Run (default port 8080)
./charmtool

# Run on different port
PORT=3000 ./charmtool
```

## Architecture

### Data Flow
1. User uploads KiCad POS file → parsed by `models/pos.go`
2. Creates **XFile** (central data structure) stored in session
3. User edits via frontend → updates XFile via `/api/xfile/update`
4. Export validates and generates DPV + STACK files as ZIP

### XFile Structure (`internal/models/xfile.go`)
The XFile is the central data model holding all converted data:
- `POSRows` - Original KiCad POS data (read-only display)
- `Components` - EComponent table (placement coordinates)
- `Stations` - Station table (feeder/material stack config)
- `PanelArray`/`PanelCoord` - Panel configuration
- `GlobalOffset` - X/Y offset applied to all coordinates on export

Components have extended fields `Select` and `DNP` (Do Not Place) not in standard DPV.

### Key Relationships
- **Station.ID** ↔ **Component.STNo** - Components reference stations by ID
- **Station.Note** = **Component.Explain** - Both hold component value (e.g., "10k")
- When Station IDs change, Component STNo references must be updated

### DPV Validation (`internal/models/dpv.go`)
Export is blocked if validation fails. Critical checks:
- Station IDs must be unique
- Component STNo must reference valid Station ID
- PHead must be 1 or 2
- Skip/Status flag consistency (vision flag mismatch causes silent machine failure)
- Height max 5mm

### Session Storage (`internal/storage/filestore.go`)
- Cookie-based sessions with 10-day expiry
- XFile stored as JSON in `data/sessions/`
- Hourly cleanup of expired sessions

### POS Parser (`internal/models/pos.go`)
Supports two formats:
- KiCad whitespace-delimited with `# Ref Val ...` header
- Standard CSV format

## DPV Format Notes

Reference: `/home/zditech/CHWebApp/DPVFileFormat.txt`

- PANELYPE=1 is current format (V1)
- No. fields must be sequential 0 to N-1 (auto-renumbered on export)
- FILE header must match actual filename exactly
- PHead in EComponent must be position 3 (after ID, before STNo)
- Coordinates are in millimeters, angles in degrees (-180 to 180)
