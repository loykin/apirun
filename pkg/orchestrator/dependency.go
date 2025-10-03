package orchestrator

import (
	"fmt"
	"sort"
)

// DependencyGraph represents a directed graph of stage dependencies
type DependencyGraph struct {
	nodes map[string]*GraphNode
	edges map[string][]string
}

// GraphNode represents a node in the dependency graph
type GraphNode struct {
	Name     string
	Stage    *Stage
	InDegree int
	Visited  bool
	InStack  bool
}

// NewDependencyGraph creates a new dependency graph
func NewDependencyGraph() *DependencyGraph {
	return &DependencyGraph{
		nodes: make(map[string]*GraphNode),
		edges: make(map[string][]string),
	}
}

// AddStage adds a stage to the dependency graph
func (dg *DependencyGraph) AddStage(stage *Stage) {
	if _, exists := dg.nodes[stage.Name]; !exists {
		dg.nodes[stage.Name] = &GraphNode{
			Name:  stage.Name,
			Stage: stage,
		}
		dg.edges[stage.Name] = []string{}
	}
}

// AddDependency adds a dependency between two stages
func (dg *DependencyGraph) AddDependency(from, to string) error {
	if _, exists := dg.nodes[from]; !exists {
		return fmt.Errorf("stage %s not found", from)
	}
	if _, exists := dg.nodes[to]; !exists {
		return fmt.Errorf("stage %s not found", to)
	}

	dg.edges[from] = append(dg.edges[from], to)
	dg.nodes[to].InDegree++
	return nil
}

// BuildGraph builds the complete dependency graph from stages
func (dg *DependencyGraph) BuildGraph(stages []Stage) error {
	// First pass: add all stages
	for i := range stages {
		dg.AddStage(&stages[i])
	}

	// Second pass: add dependencies
	for _, stage := range stages {
		for _, dep := range stage.DependsOn {
			if err := dg.AddDependency(dep, stage.Name); err != nil {
				return fmt.Errorf("failed to add dependency %s -> %s: %w", dep, stage.Name, err)
			}
		}
	}

	return nil
}

// DetectCycle detects circular dependencies using DFS
func (dg *DependencyGraph) DetectCycle() []string {
	// Reset visit state
	for _, node := range dg.nodes {
		node.Visited = false
		node.InStack = false
	}

	for name, node := range dg.nodes {
		if !node.Visited {
			if cycle := dg.dfsDetectCycle(name, []string{}); cycle != nil {
				return cycle
			}
		}
	}
	return nil
}

func (dg *DependencyGraph) dfsDetectCycle(nodeName string, path []string) []string {
	node := dg.nodes[nodeName]
	node.Visited = true
	node.InStack = true
	path = append(path, nodeName)

	for _, neighbor := range dg.edges[nodeName] {
		neighborNode := dg.nodes[neighbor]
		if neighborNode.InStack {
			// Found cycle - return the cycle path
			cycleStart := -1
			for i, name := range path {
				if name == neighbor {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				return append(path[cycleStart:], neighbor)
			}
		}
		if !neighborNode.Visited {
			if cycle := dg.dfsDetectCycle(neighbor, path); cycle != nil {
				return cycle
			}
		}
	}

	node.InStack = false
	return nil
}

// TopologicalSort returns stages in execution order using Kahn's algorithm
func (dg *DependencyGraph) TopologicalSort() ([]string, error) {
	// Check for cycles first
	if cycle := dg.DetectCycle(); cycle != nil {
		return nil, fmt.Errorf("circular dependency detected: %v", cycle)
	}

	// Create a copy of in-degrees for processing
	inDegree := make(map[string]int)
	for name, node := range dg.nodes {
		inDegree[name] = node.InDegree
	}

	// Find all nodes with no incoming edges
	queue := []string{}
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	// Sort queue for deterministic output
	sort.Strings(queue)

	result := []string{}
	for len(queue) > 0 {
		// Process next batch (nodes with no dependencies)
		currentBatch := queue
		queue = []string{}

		// Sort current batch for deterministic output
		sort.Strings(currentBatch)

		for _, current := range currentBatch {
			result = append(result, current)

			// Reduce in-degree of neighbors
			for _, neighbor := range dg.edges[current] {
				inDegree[neighbor]--
				if inDegree[neighbor] == 0 {
					queue = append(queue, neighbor)
				}
			}
		}
	}

	// Verify all nodes were processed
	if len(result) != len(dg.nodes) {
		return nil, fmt.Errorf("topological sort failed - graph contains cycles")
	}

	return result, nil
}

// GetBatches returns stages grouped by execution batches (parallel execution within batches)
func (dg *DependencyGraph) GetBatches() ([][]string, error) {
	if cycle := dg.DetectCycle(); cycle != nil {
		return nil, fmt.Errorf("circular dependency detected: %v", cycle)
	}

	inDegree := make(map[string]int)
	for name, node := range dg.nodes {
		inDegree[name] = node.InDegree
	}

	batches := [][]string{}
	processed := make(map[string]bool)

	for len(processed) < len(dg.nodes) {
		// Find all nodes that can be executed in this batch
		currentBatch := []string{}
		for name, degree := range inDegree {
			if degree == 0 && !processed[name] {
				currentBatch = append(currentBatch, name)
			}
		}

		if len(currentBatch) == 0 {
			return nil, fmt.Errorf("unable to find next batch - possible circular dependency")
		}

		// Sort for deterministic output
		sort.Strings(currentBatch)
		batches = append(batches, currentBatch)

		// Mark as processed and update in-degrees
		for _, name := range currentBatch {
			processed[name] = true
			for _, neighbor := range dg.edges[name] {
				inDegree[neighbor]--
			}
		}
	}

	return batches, nil
}

// GetStagesByNames returns stages by their names in the given order
func (dg *DependencyGraph) GetStagesByNames(names []string) []*Stage {
	stages := make([]*Stage, 0, len(names))
	for _, name := range names {
		if node, exists := dg.nodes[name]; exists {
			stages = append(stages, node.Stage)
		}
	}
	return stages
}

// GetDependents returns all stages that depend on the given stage
func (dg *DependencyGraph) GetDependents(stageName string) []string {
	if deps, exists := dg.edges[stageName]; exists {
		// Return a copy to avoid modifying the original slice
		result := make([]string, len(deps))
		copy(result, deps)
		return result
	}
	return []string{}
}

// GetAllDependents returns all stages that depend on the given stage, recursively
func (dg *DependencyGraph) GetAllDependents(stageName string) []string {
	visited := make(map[string]bool)
	result := make([]string, 0)

	var collect func(string)
	collect = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true

		// Get direct dependents
		if deps, exists := dg.edges[name]; exists {
			for _, dep := range deps {
				result = append(result, dep)
				collect(dep)
			}
		}
	}

	collect(stageName)
	return result
}
