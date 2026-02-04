# CharmTool Web

A Go-based web application that converts KiCad POS placement files to Charmhigh CHMT-48VB pick-and-place machine DPV format.

## Quick Start

```bash
# Build the server
go build -o charmtool ./cmd/server

# Run the server
./charmtool

# Open in browser
# http://localhost:8080
```

## Project Structure

```
CharmToolWeb/
├── cmd/server/main.go           # Server entry point
├── internal/
│   ├── handlers/
│   │   ├── handlers.go          # API route handlers
│   │   └── session.go           # Session middleware
│   ├── models/
│   │   ├── xfile.go             # X file data structure
│   │   ├── dpv.go               # DPV generation & validation
│   │   ├── pos.go               # POS/CSV parsing
│   │   └── stack.go             # STACK file handling
│   └── storage/
│       └── filestore.go         # Session file storage
├── web/static/index.html        # Single-page frontend
├── data/sessions/               # Runtime session storage (gitignored)
└── go.mod
```

## Features

- **Load KiCad POS files** - Parses CSV/POS placement exports
- **Material Stack management** - Configure feeders, visual parameters, nozzle assignments
- **STACK file merge** - Load saved feeder configurations
- **DPV validation** - Comprehensive validation per machine specification before export
- **Export ZIP package** - Contains DPV file and Stack backup
- **Session-based storage** - 10-day session persistence with automatic cleanup

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/upload/pos` | POST | Upload KiCad POS file |
| `/api/upload/stack` | POST | Upload and merge STACK file |
| `/api/xfile` | GET | Get current session X file |
| `/api/xfile/update` | POST | Update X file from client |
| `/api/validate` | GET | Validate DPV before export |
| `/api/export` | GET | Download ZIP (DPV + Stack) |

## DPV Validation

Before export, the following validations are performed per DPVFileFormat.txt specification:

- Station IDs are unique
- Component STNo. references valid Station IDs
- PHead values are 1 or 2
- Station/Component Status/Skip flag consistency (vision flag)
- Height values within machine limits (max 5mm)
- Panel array configuration validity
- Sequential No. fields (renumbered on export)
- FILE header matches output filename

## Configuration

Environment variables:
- `PORT` - Server port (default: 8080)

Session storage:
- Sessions persist for 10 days
- Cleanup runs hourly
- Data stored in `data/sessions/`

## License

Copyright 2026 Rick McNeely
