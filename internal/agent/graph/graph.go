package graph

type Plan struct {
	Nodes []NodeSpec
}

type NodeSpec struct {
	ID        string
	Skill     string
	DependsOn []string
}
