package orchestrator

import "context"

// Delete removes a document by ID, delegating to the persistence gateway.
func Delete(ctx context.Context, p Ports, id string) error {
	return p.Persistence.Delete(ctx, id)
}
