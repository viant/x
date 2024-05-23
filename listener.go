package x

// Listener represents a listener
type Listener func(t *Type)

// MergeListener represents a merge listener
type MergeListener func() Listener
