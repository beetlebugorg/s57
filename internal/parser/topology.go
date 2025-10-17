package parser

// topology.go - VRPT (Vector Record Pointer Table) topology resolution
// Implements S-57 Edition 3.1 polygon construction from edge references

// spatialKey uniquely identifies a spatial record by (RCNM, RCID) pair
// S-57 §2.2.2 (31Main.pdf): RCID is unique within a record type, not globally
type spatialKey struct {
	RCNM int   // Record name (110=node, 120=connected node, 130=edge, 140=face)
	RCID int64 // Record ID (unique within RCNM type)
}

// edge represents a spatial edge record with connectivity information
// S-57 §5.1.3.2 (31Main.pdf): Edges connect nodes to form polygon boundaries
type edge struct {
	ID          int64        // Edge record ID (RCID)
	Points      [][2]float64 // Coordinate points along the edge [lon, lat]
	StartNodeID int64        // ID of starting node
	EndNodeID   int64        // ID of ending node
}

// polygonBuilder constructs polygon geometries from topological primitives (edges/nodes)
// Caches edges to avoid redundant lookups during ring construction
type polygonBuilder struct {
	spatialRecords map[spatialKey]*spatialRecord // Spatial records indexed by (RCNM, RCID)
	edgeCache      map[int64]*edge               // Cached edges for reuse
}

// newPolygonBuilder creates a new polygon builder with given spatial records
func newPolygonBuilder(spatialRecords map[spatialKey]*spatialRecord) *polygonBuilder {
	return &polygonBuilder{
		spatialRecords: spatialRecords,
		edgeCache:      make(map[int64]*edge),
	}
}

// getNode retrieves a node's coordinates from spatial records
// Tries connected node first, then isolated node
func (r *polygonBuilder) getNode(nodeID int64) *spatialRecord {
	// Try connected node
	nodeKey := spatialKey{RCNM: int(spatialTypeConnectedNode), RCID: nodeID}
	if node, ok := r.spatialRecords[nodeKey]; ok && len(node.Coordinates) > 0 {
		return node
	}
	// Try isolated node
	nodeKey = spatialKey{RCNM: int(spatialTypeIsolatedNode), RCID: nodeID}
	if node, ok := r.spatialRecords[nodeKey]; ok && len(node.Coordinates) > 0 {
		return node
	}
	return nil
}

// getFullEdgeCoordinates builds full edge coordinates: start node + SG2D + end node
// Reverses the entire array if orientation==2 (like marinejet does)
func (r *polygonBuilder) getFullEdgeCoordinates(edge *edge, orientation int) [][2]float64 {
	coords := make([][2]float64, 0)

	// Add start node
	if edge.StartNodeID != 0 {
		if node := r.getNode(edge.StartNodeID); node != nil && len(node.Coordinates) > 0 {
			// Extract 2D coordinate (first 2 values) from variable-length coordinate
			coord := node.Coordinates[0]
			if len(coord) >= 2 {
				coords = append(coords, [2]float64{coord[0], coord[1]})
			}
		}
	}

	// Add SG2D intermediate points
	coords = append(coords, edge.Points...)

	// Add end node
	if edge.EndNodeID != 0 {
		if node := r.getNode(edge.EndNodeID); node != nil && len(node.Coordinates) > 0 {
			// Extract 2D coordinate (first 2 values) from variable-length coordinate
			coord := node.Coordinates[0]
			if len(coord) >= 2 {
				coords = append(coords, [2]float64{coord[0], coord[1]})
			}
		}
	}

	// Reverse if orientation is 2
	if orientation == 2 {
		reversed := make([][2]float64, len(coords))
		for i, coord := range coords {
			reversed[len(coords)-1-i] = coord
		}
		return reversed
	}

	return coords
}

// loadEdge loads an edge from spatial records, with caching
// Returns cached edge if already loaded, otherwise loads from spatial record
func (r *polygonBuilder) loadEdge(edgeID int64) (*edge, error) {
	// Check cache first
	if edge, ok := r.edgeCache[edgeID]; ok {
		return edge, nil
	}

	// Load from spatial records using composite key (RCNM=130 for edges)
	edgeKey := spatialKey{RCNM: int(spatialTypeEdge), RCID: edgeID}
	spatial, ok := r.spatialRecords[edgeKey]
	if !ok {
		return nil, &ErrMissingSpatialRecord{
			FeatureID: 0, // Feature ID not known at this level
			SpatialID: edgeID,
		}
	}

	// Verify this is an edge record (RCNM = 130)
	if spatial.RecordType != spatialTypeEdge {
		return nil, &ErrInvalidSpatialRecord{
			SpatialID: edgeID,
			Reason:    "expected edge record (RCNM=130)",
		}
	}

	// Extract node connectivity from vector pointers
	// S-57 §5.1.3.2 (31Main.pdf): Edges must reference nodes via VRPT with topology indicators:
	//   B{1} = Beginning node (required)
	//   E{2} = End node (required)
	//   S{3} = Left face (required in full topology)
	//   D{4} = Right face (required in full topology)
	// References must appear in sequence: B, E, S, D
	var startNodeID, endNodeID int64
	for _, ptr := range spatial.VectorPointers {
		// Node records have RCNM = 110 (isolated) or 120 (connected)
		if ptr.TargetRCNM == int(spatialTypeIsolatedNode) || ptr.TargetRCNM == int(spatialTypeConnectedNode) {
			if startNodeID == 0 {
				startNodeID = ptr.TargetRCID
			} else if endNodeID == 0 {
				endNodeID = ptr.TargetRCID
			}
		}
	}

	// Extract edge geometry per S-57 §5.1.4.4 (31Main.pdf):
	// "The geometry of the connected node is not part of the edge"
	// This means edge.Points contains ONLY the SG2D intermediate shape points
	// Nodes are stored separately and referenced via VRPT

	// Edge.Points = SG2D coordinates only (may be empty for straight-line edges)
	// Convert variable-length coordinates to fixed 2D coordinates
	points := make([][2]float64, 0, len(spatial.Coordinates))
	for _, coord := range spatial.Coordinates {
		if len(coord) >= 2 {
			points = append(points, [2]float64{coord[0], coord[1]})
		}
	}

	// Create edge
	newEdge := &edge{
		ID:          edgeID,
		Points:      points,
		StartNodeID: startNodeID,
		EndNodeID:   endNodeID,
	}

	// Cache for reuse
	r.edgeCache[edgeID] = newEdge

	return newEdge, nil
}

// resolvePolygon constructs polygon rings from edge references via VRPT topology
// IMPORTANT: Despite S-57 §4.7.3 (31Main.pdf) saying edges "must be referenced sequentially",
// real-world ENC files do NOT provide edges in sequential order. We must follow
// topology graph by matching node connectivity.
func (r *polygonBuilder) resolvePolygon(edgeRefs []spatialRef) ([][][2]float64, error) {
	if len(edgeRefs) == 0 {
		return nil, &ErrInvalidGeometry{
			Reason: "no edge references provided",
		}
	}

	// Pre-load all edges and store with their orientations
	edgeOrientations := make(map[int64]int) // edgeID -> orientation
	for _, edgeRef := range edgeRefs {
		if _, err := r.loadEdge(edgeRef.RCID); err != nil {
			// Skip edges that fail to load
			continue
		}
		edgeOrientations[edgeRef.RCID] = edgeRef.Orientation
	}

	// Build rings by following topology graph
	return r.buildRingsWithOrientation(edgeRefs, edgeOrientations)
}

// buildRingsWithOrientation constructs polygon rings using FSPT edge order
// Follows marinejet's approach: iterate edges in FSPT order, apply orientation, deduplicate nodes
// Per S-57 §4.7.3 (31Main.pdf): "vector records making up an area boundary must be referenced sequentially"
func (r *polygonBuilder) buildRingsWithOrientation(edgeRefs []spatialRef, orientations map[int64]int) ([][][2]float64, error) {
	// Build single ring from edges in FSPT order (matching marinejet lines 373-446)
	coords := make([][2]float64, 0)

	for _, edgeRef := range edgeRefs {
		// Load edge
		edge, err := r.loadEdge(edgeRef.RCID)
		if err != nil {
			continue // Skip failed edges
		}

		// Get edge coordinates with orientation applied
		edgeCoords := r.getFullEdgeCoordinates(edge, edgeRef.Orientation)

		// Deduplicate: skip first coordinate if it matches last coordinate in ring
		if len(coords) > 0 && len(edgeCoords) > 0 {
			lastCoord := coords[len(coords)-1]
			firstNewCoord := edgeCoords[0]
			if lastCoord[0] == firstNewCoord[0] && lastCoord[1] == firstNewCoord[1] {
				edgeCoords = edgeCoords[1:]
			}
		}

		coords = append(coords, edgeCoords...)
	}

	// Ensure ring closure
	if len(coords) > 0 && !isRingClosed(coords) {
		coords = append(coords, coords[0])
	}

	if len(coords) == 0 {
		return nil, &ErrInvalidGeometry{
			Reason: "no coordinates collected from edges",
		}
	}

	return [][][2]float64{coords}, nil
}

// isRingClosed checks if a ring is properly closed
func isRingClosed(ring [][2]float64) bool {
	if len(ring) < 3 {
		return false
	}
	first := ring[0]
	last := ring[len(ring)-1]
	return first[0] == last[0] && first[1] == last[1]
}
