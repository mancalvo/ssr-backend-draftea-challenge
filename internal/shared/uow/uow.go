// Package uow provides the Unit of Work pattern for transaction management.
// This is a domain-level interface that keeps usecases clean of database dependencies.
package uow

import "context"

// TransactionRunner executes operations within a database transaction.
// Commits on success, rolls back on error or panic.
type TransactionRunner interface {
	// RunInTransaction executes fn within a transaction.
	// The context passed to fn contains the transaction for repositories to use.
	RunInTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}
