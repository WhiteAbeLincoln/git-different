package dirnode

type DirNode struct {
	FullPath string
	Name     string
}

// DirNode implements fmt.Stringer which charm.land/bubbles uses to render it in the tree bubble.
func (d *DirNode) String() string {
	return d.Name
}

// CommitNode represents a commit header in the segmented file tree view.
type CommitNode struct {
	Hash    string
	Subject string
}

func (c *CommitNode) String() string {
	hash := c.Hash
	if len(hash) > 7 {
		hash = hash[:7]
	}
	return hash + " " + c.Subject
}
