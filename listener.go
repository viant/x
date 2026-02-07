package x

// Listener is invoked on registration events.
//
// When used with Register (single-type), the registry's configured Listener
// is called with the registered type.
// When used during Merge (multi-type), the effective listener is the one
// returned by the MergeListener factory (if configured) and is called once
// per merged type. See MergeListener for end-of-merge signalling.
type Listener func(t *Type)

// MergeListener is a factory for a per-merge Listener.
//
// Merge obtains a fresh Listener by calling the MergeListener at the start of
// the merge, then invokes the returned Listener for each merged type. After
// all types have been processed, the returned Listener is called once with a
// nil *Type to signal end-of-merge. Implementations MUST tolerate a nil
// argument and treat it as a completion notification.
type MergeListener func() Listener
