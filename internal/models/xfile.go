package models

import "time"

// XFile is the central data structure that holds all converted data
type XFile struct {
	Metadata     XFileMetadata   `json:"metadata"`
	GlobalOffset GlobalOffset    `json:"globalOffset"`
	POSRows      []POSRow        `json:"posRows"`      // Original POS file data
	Components   []XComponent    `json:"components"`
	Stations     []XStation      `json:"stations"`
	PanelArray   []PanelArrayRow `json:"panelArray"`
	PanelCoord   []PanelCoordRow `json:"panelCoord"`
	OriginalPOS  string          `json:"originalPOS"`  // Original POS filename
	StackFiles   []string        `json:"stackFiles"`   // Loaded STACK filenames
}

// POSRow represents a single row from the original KiCad POS file
type POSRow struct {
	Ref     string  `json:"ref"`
	Val     string  `json:"val"`
	Package string  `json:"package"`
	PosX    float64 `json:"posx"`
	PosY    float64 `json:"posy"`
	Rot     float64 `json:"rot"`
	Side    string  `json:"side"`
}

// XFileMetadata contains file metadata
type XFileMetadata struct {
	Created  time.Time `json:"created"`
	Modified time.Time `json:"modified"`
}

// GlobalOffset contains X/Y offset applied to all component positions
type GlobalOffset struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// XComponent represents a component placement (EComponent table row)
// Extended with Select and DNP fields not in standard DPV
type XComponent struct {
	// Standard DPV EComponent fields
	No      int     `json:"no"`      // Row number (0 to N-1)
	ID      int     `json:"id"`      // Component number
	PHead   int     `json:"phead"`   // Nozzle number (1 or 2)
	STNo    int     `json:"stno"`    // Station number reference
	DeltX   float64 `json:"deltx"`   // X position on board
	DeltY   float64 `json:"delty"`   // Y position on board
	Angle   float64 `json:"angle"`   // Rotation angle
	Height  float64 `json:"height"`  // Part height
	Skip    int     `json:"skip"`    // Skip flags (0=place, 1=skip, 4=vision)
	Speed   int     `json:"speed"`   // Transport speed %
	Explain string  `json:"explain"` // Component value (Val)
	Note    string  `json:"note"`    // Part designation (Ref - Package)
	Delay   int     `json:"delay"`   // Delay before pickup (cs)

	// Extended fields (not in standard DPV)
	Select bool `json:"select"` // UI selection state
	DNP    bool `json:"dnp"`    // Do Not Place flag
}

// XStation represents a material stack/feeder (Station table row)
// Extended with Select, PHead, and DNP fields
type XStation struct {
	// Standard DPV Station fields
	No              int     `json:"no"`              // Row number (0 to N-1)
	ID              int     `json:"id"`              // Station/feeder ID
	DeltX           float64 `json:"deltx"`           // X pocket offset
	DeltY           float64 `json:"delty"`           // Y pocket offset
	FeedRates       int     `json:"feedrates"`       // Feed distance (2, 4, 8)
	Note            string  `json:"note"`            // Part value/description
	Height          float64 `json:"height"`          // Part height
	Speed           int     `json:"speed"`           // Transport speed %
	Status          int     `json:"status"`          // Flags (1=skip, 2=vacuum, 4=vision, 8=pause)
	NPixSizeX       int     `json:"npixsizex"`       // Visual X size (pixels)
	NPixSizeY       int     `json:"npixsizey"`       // Visual Y size (pixels)
	HeightTake      float64 `json:"heighttake"`      // Unknown
	DelayTake       int     `json:"delaytake"`       // Pickup delay (cs)
	NPullStripSpeed int     `json:"npullstripspeed"` // Tape pull speed %
	NThreshold      int     `json:"nthreshold"`      // Visual threshold
	NVisualRadio    int     `json:"nvisualradio"`    // Visual ratio %

	// Extended fields (not in standard DPV)
	Select bool `json:"select"` // UI selection state
	PHead  int  `json:"phead"`  // Preferred nozzle (1 or 2)
	DNP    bool `json:"dnp"`    // Do Not Place flag
}

// PanelArrayRow represents a Panel_Array table row
type PanelArrayRow struct {
	No        int     `json:"no"`
	ID        int     `json:"id"`        // 1=config row, N=skip board N
	IntervalX float64 `json:"intervalx"` // X distance between columns
	IntervalY float64 `json:"intervaly"` // Y distance between rows
	NumX      int     `json:"numx"`      // Number of columns
	NumY      int     `json:"numy"`      // Number of rows
}

// PanelCoordRow represents a Panel_Coord table row
type PanelCoordRow struct {
	No    int     `json:"no"`
	ID    int     `json:"id"`
	DeltX float64 `json:"deltx"` // X offset to board 0,0
	DeltY float64 `json:"delty"` // Y offset to board 0,0
}

// NewXFile creates a new empty XFile with defaults
func NewXFile() *XFile {
	now := time.Now()
	return &XFile{
		Metadata: XFileMetadata{
			Created:  now,
			Modified: now,
		},
		GlobalOffset: GlobalOffset{X: 0, Y: 0},
		POSRows:      []POSRow{},
		Components:   []XComponent{},
		Stations:     []XStation{},
		PanelArray: []PanelArrayRow{
			{No: 0, ID: 1, IntervalX: 0, IntervalY: 0, NumX: 1, NumY: 1},
		},
		PanelCoord: []PanelCoordRow{
			{No: 0, ID: 1, DeltX: 0, DeltY: 0},
		},
		OriginalPOS: "",
		StackFiles:  []string{},
	}
}
