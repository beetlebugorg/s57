package parser

import (
	"fmt"
	"os"
)

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
			coords = append(coords, node.Coordinates[0])
		}
	}

	// Add SG2D intermediate points
	coords = append(coords, edge.Points...)

	// Add end node
	if edge.EndNodeID != 0 {
		if node := r.getNode(edge.EndNodeID); node != nil && len(node.Coordinates) > 0 {
			coords = append(coords, node.Coordinates[0])
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
	points := make([][2]float64, len(spatial.Coordinates))
	copy(points, spatial.Coordinates)

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

	for idx, edgeRef := range edgeRefs {
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

		// Debug for large rings
		if len(edgeRefs) >= 20 && (idx < 5 || idx == len(edgeRefs)-1) {
			fmt.Fprintf(os.Stderr, "  [%d] Edge %d: added %d coords, total=%d\n",
				idx, edgeRef.RCID, len(edgeCoords), len(coords))
		}
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

// followRingFromEdgeWithOrientation follows edges by connectivity, applying orientations
func (r *polygonBuilder) followRingFromEdgeWithOrientation(startEdgeID int64, orientations map[int64]int, visited map[int64]bool) ([][2]float64, []int64, error) {
	ring := make([][2]float64, 0)
	edgesUsed := make([]int64, 0)

	// Load starting edge
	currentEdge, err := r.loadEdge(startEdgeID)
	if err != nil {
		return nil, nil, err
	}

	// Get orientation for starting edge
	orientation := orientations[startEdgeID]

	// Build coordinates for first edge with orientation (marinejet approach)
	ring = r.getFullEdgeCoordinates(currentEdge, orientation)

	edgesUsed = append(edgesUsed, currentEdge.ID)
	visited[currentEdge.ID] = true

	// Determine where we are now based on orientation
	var currentNodeID, startNodeID int64
	if orientation == 2 {
		// Reversed edge: we end at start node, started at end node
		currentNodeID = currentEdge.StartNodeID
		startNodeID = currentEdge.EndNodeID
	} else {
		// Forward edge: we end at end node, started at start node
		currentNodeID = currentEdge.EndNodeID
		startNodeID = currentEdge.StartNodeID
	}

	// Follow connected edges
	return r.followChainForward(currentNodeID, startNodeID, ring, edgesUsed, orientations, visited)
}

// followChainForward follows edge chain until returning to start node
func (r *polygonBuilder) followChainForward(currentNodeID, startNodeID int64, ring [][2]float64, edgesUsed []int64, orientations map[int64]int, visited map[int64]bool) ([][2]float64, []int64, error) {
	maxIterations := 10000
	iterations := 0

	for currentNodeID != startNodeID && iterations < maxIterations {
		iterations++

		// Find next connected edge
		nextEdge, nextOrientation, err := r.findConnectedEdgeWithOrientation(currentNodeID, orientations, visited)
		if err != nil {
			return ring, edgesUsed, nil // Return what we have
		}

		// Build full edge coordinates: start node + SG2D + end node
		// Then reverse if needed (marinejet approach)
		edgeCoords := r.getFullEdgeCoordinates(nextEdge, nextOrientation)

		// Deduplicate: skip first coord if it equals last coord in ring
		if len(ring) > 0 && len(edgeCoords) > 0 {
			lastRingCoord := ring[len(ring)-1]
			firstEdgeCoord := edgeCoords[0]
			if lastRingCoord[0] == firstEdgeCoord[0] && lastRingCoord[1] == firstEdgeCoord[1] {
				edgeCoords = edgeCoords[1:]
			}
		}

		ring = append(ring, edgeCoords...)

		// Update current node based on orientation
		if nextOrientation == 2 {
			currentNodeID = nextEdge.StartNodeID // Reversed edge ends at start node
		} else {
			currentNodeID = nextEdge.EndNodeID // Forward edge ends at end node
		}

		edgesUsed = append(edgesUsed, nextEdge.ID)
		visited[nextEdge.ID] = true
	}

	if iterations >= maxIterations {
		return nil, nil, &ErrInvalidGeometry{
			Reason: "ring construction exceeded maximum iterations",
		}
	}

	return ring, edgesUsed, nil
}

// findConnectedEdgeWithOrientation finds next edge connected to given node with its orientation
// The key insight: FSPT ORNT tells us which end of the edge we START from
// - ORNT=Forward(1): we enter edge at its Start node, exit at End node
// - ORNT=Reverse(2): we enter edge at its End node, exit at Start node
func (r *polygonBuilder) findConnectedEdgeWithOrientation(nodeID int64, orientations map[int64]int, visited map[int64]bool) (*edge, int, error) {
	for edgeID, edge := range r.edgeCache {
		if visited[edgeID] {
			continue
		}

		fsptOrient := orientations[edgeID]

		// Check if this edge's entry point matches our current node
		if fsptOrient != 2 {
			// Forward orientation: edge entry point is StartNode
			if edge.StartNodeID == nodeID {
				return edge, fsptOrient, nil
			}
		} else {
			// Reverse orientation: edge entry point is EndNode
			if edge.EndNodeID == nodeID {
				return edge, fsptOrient, nil
			}
		}
	}

	return nil, 0, &ErrInvalidGeometry{
		Reason: fmt.Sprintf("no connected edge found for node %d", nodeID),
	}
}

// buildRings constructs closed polygon rings from edge references
// S-57 §4.7.3.1 (31Main.pdf): Edges connect via nodes to form closed boundaries
func (r *polygonBuilder) buildRings(edgeRefs []int64) ([][][2]float64, error) {
	// Pre-load all edges into cache
	for _, edgeID := range edgeRefs {
		if _, err := r.loadEdge(edgeID); err != nil {
			return nil, err
		}
	}

	rings := make([][][2]float64, 0)
	visited := make(map[int64]bool)

	for _, edgeID := range edgeRefs {
		if visited[edgeID] {
			continue // Already processed in a ring
		}

		// Start new ring from this edge
		ring, edgesUsed, err := r.followRingFromEdge(edgeID, visited)
		if err != nil {
			return nil, err
		}

		// Mark all edges in this ring as visited
		for _, usedEdge := range edgesUsed {
			visited[usedEdge] = true
		}

		// Validate ring closure
		if !isRingClosed(ring) {
			return nil, &ErrInvalidGeometry{
				Reason: "ring is not closed (first point != last point)",
			}
		}

		rings = append(rings, ring)
	}

	return rings, nil
}

// followRingFromEdge follows edges from a starting edge until ring closure
// Returns the constructed ring and list of edge IDs used
func (r *polygonBuilder) followRingFromEdge(startEdgeID int64, visited map[int64]bool) ([][2]float64, []int64, error) {
	ring := make([][2]float64, 0)
	edgesUsed := make([]int64, 0)

	// Load starting edge
	currentEdge, err := r.loadEdge(startEdgeID)
	if err != nil {
		return nil, nil, err
	}

	// Add starting edge points
	ring = append(ring, currentEdge.Points...)
	edgesUsed = append(edgesUsed, currentEdge.ID)
	visited[currentEdge.ID] = true // Mark starting edge as visited
	currentNodeID := currentEdge.EndNodeID
	startNodeID := currentEdge.StartNodeID

	// Follow connected edges until we return to start node
	maxIterations := 10000 // Prevent infinite loops
	iterations := 0

	for currentNodeID != startNodeID && iterations < maxIterations {
		iterations++

		// Find next connected edge
		nextEdge, reverse, err := r.findConnectedEdge(currentNodeID, visited)
		if err != nil {
			return nil, nil, err
		}

		// Add edge points (reversed if needed)
		if reverse {
			// Add points in reverse order, skipping first (shared node)
			for i := len(nextEdge.Points) - 2; i >= 0; i-- {
				ring = append(ring, nextEdge.Points[i])
			}
			currentNodeID = nextEdge.StartNodeID
		} else {
			// Add points in forward order, skipping first (shared node)
			for i := 1; i < len(nextEdge.Points); i++ {
				ring = append(ring, nextEdge.Points[i])
			}
			currentNodeID = nextEdge.EndNodeID
		}

		edgesUsed = append(edgesUsed, nextEdge.ID)
		// Mark as visited locally to prevent reuse within this ring
		visited[nextEdge.ID] = true
	}

	if iterations >= maxIterations {
		return nil, nil, &ErrInvalidGeometry{
			Reason: "ring construction exceeded maximum iterations (possible topology error)",
		}
	}

	// Ensure ring closure (first point == last point)
	if len(ring) > 0 {
		first := ring[0]
		last := ring[len(ring)-1]
		if first[0] != last[0] || first[1] != last[1] {
			ring = append(ring, first) // Close the ring
		}
	}

	return ring, edgesUsed, nil
}

// findConnectedEdge finds the next edge connected to the given node
// Returns the edge, whether it should be reversed, and any error
func (r *polygonBuilder) findConnectedEdge(nodeID int64, visited map[int64]bool) (*edge, bool, error) {
	// Search all edges for one that connects to this node
	for edgeID, edge := range r.edgeCache {
		if visited[edgeID] {
			continue
		}

		if edge.StartNodeID == nodeID {
			return edge, false, nil // Use edge in forward direction
		}
		if edge.EndNodeID == nodeID {
			return edge, true, nil // Use edge in reverse direction
		}
	}

	// Edge not in cache, search spatial records
	for _, spatial := range r.spatialRecords {
		if visited[spatial.ID] || spatial.RecordType != spatialTypeEdge {
			continue
		}

		// Load edge to check connectivity
		edge, err := r.loadEdge(spatial.ID)
		if err != nil {
			continue // Skip edges that can't be loaded
		}

		if edge.StartNodeID == nodeID {
			return edge, false, nil
		}
		if edge.EndNodeID == nodeID {
			return edge, true, nil
		}
	}

	return nil, false, &ErrInvalidGeometry{
		Reason: "no connected edge found for node",
	}
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

// classifyRings classifies rings as outer (clockwise) or inner (counter-clockwise)
// S-57 §4.7.3.2 (31Main.pdf): Outer rings clockwise, inner rings counter-clockwise
func classifyRings(rings [][][2]float64) [][][2]float64 {
	classified := make([][][2]float64, 0, len(rings))

	for _, ring := range rings {
		area := signedArea(ring)

		// S-57 convention: positive area = clockwise = outer ring
		// Negative area = counter-clockwise = inner ring (hole)
		if area > 0 {
			// Outer ring - use as-is
			classified = append(classified, ring)
		} else {
			// Inner ring (hole) - reverse to follow outer ring convention
			// (Or keep as-is depending on rendering requirements)
			classified = append(classified, ring)
		}
	}

	return classified
}

// signedArea calculates the signed area of a polygon ring
// Positive area = clockwise, negative area = counter-clockwise
// Uses shoelace formula: A = 0.5 * Σ(x_i * y_{i+1} - x_{i+1} * y_i)
func signedArea(ring [][2]float64) float64 {
	if len(ring) < 3 {
		return 0
	}

	area := 0.0
	n := len(ring)

	for i := 0; i < n-1; i++ {
		x1, y1 := ring[i][0], ring[i][1]
		x2, y2 := ring[i+1][0], ring[i+1][1]
		area += (x1 * y2) - (x2 * y1)
	}

	return area / 2.0
}
