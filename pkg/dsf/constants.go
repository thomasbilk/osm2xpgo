package dsf

// DSF atom identifiers as little-endian uint32 values.
// Each constant corresponds to a 4-character ASCII atom name stored on disk
// in little-endian byte order.
const (
	AtomHEAD uint32 = 0x44414548 // "HEAD"
	AtomPROP uint32 = 0x504F5250 // "PROP"
	AtomDEFN uint32 = 0x4E464544 // "DEFN"
	AtomTERT uint32 = 0x54524554 // "TERT"
	AtomOBJT uint32 = 0x544A424F // "OBJT"
	AtomPOLY uint32 = 0x594C4F50 // "POLY"
	AtomNETW uint32 = 0x5754454E // "NETW"
	AtomDEMN uint32 = 0x4E4D4544 // "DEMN"
	AtomGEOD uint32 = 0x444F4547 // "GEOD"
	AtomPOOL uint32 = 0x4C4F4F50 // "POOL"
	AtomSCAL uint32 = 0x4C414353 // "SCAL"
	AtomPO32 uint32 = 0x32334F50 // "PO32"
	AtomSC32 uint32 = 0x32334353 // "SC32"
	AtomCMDS uint32 = 0x53444D43 // "CMDS"
)

// DSF command opcodes used in the CMDS atom to reference building blocks.
const (
	CmdPoolSelect     uint8 = 1  // Select active point pool
	CmdJunctionOffset uint8 = 2  // Set junction offset
	CmdSetDefinition  uint8 = 3  // Set active terrain/object/polygon/network definition
	CmdSetRoadSubType uint8 = 4  // Set road sub-type
	CmdObject         uint8 = 5  // Place object at point pool index
	CmdObjectRange    uint8 = 6  // Place objects from index range
	CmdNetworkChain   uint8 = 7  // Network chain (sequence of point indices)
	CmdNetworkRange   uint8 = 8  // Network chain by index range
	CmdPolygon        uint8 = 9  // Polygon (single winding)
	CmdPolygonRange   uint8 = 10 // Polygon by index range
	CmdTerrainPatch   uint8 = 11 // Begin terrain patch
	CmdTerrainPatchC  uint8 = 12 // Begin terrain patch with flags
	CmdTriangleFan    uint8 = 13 // Triangle fan indices
	CmdTriangleFanC   uint8 = 14 // Triangle fan cross-pool
	CmdTriangleFanR   uint8 = 15 // Triangle fan range
	CmdComment        uint8 = 16 // Comment (ignored)
	CmdPolygonWinding uint8 = 17 // Begin new polygon winding
)

// Cookie is the 8-byte file header literal identifying a valid DSF file.
var Cookie = [8]byte{'X', 'P', 'L', 'N', 'E', 'D', 'S', 'F'}

// DSFVersion is the version number written after the cookie in DSF files.
const DSFVersion uint32 = 1
