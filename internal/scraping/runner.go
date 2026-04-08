package scraping

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

const maxConcurrent = 3

// Runner orchestrates scrape workers for all active stores.
type Runner struct {
	db     *sql.DB
	worker *Worker
	repo   *Repository
}

// NewRunner constructs a Runner.
func NewRunner(db *sql.DB, worker *Worker) *Runner {
	return &Runner{
		db:     db,
		worker: worker,
		repo:   &Repository{},
	}
}

// RunAll loads all active stores and runs their workers concurrently,
// limited to maxConcurrent (3) simultaneous workers.
func (r *Runner) RunAll(ctx context.Context) error {
	stores, err := r.repo.LoadActiveStoresWithRules(ctx, r.db)
	if err != nil {
		return fmt.Errorf("loading stores: %w", err)
	}

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	errs := make([]error, len(stores))

	for i, sw := range stores {
		wg.Add(1)
		sem <- struct{}{} // acquire
		go func(idx int, store StoreWithRule) {
			defer wg.Done()
			defer func() { <-sem }() // release
			if runErr := r.worker.Run(ctx, store); runErr != nil {
				errs[idx] = fmt.Errorf("store %q: %w", store.Store.Name, runErr)
			}
		}(i, sw)
	}

	wg.Wait()

	// Collect non-nil errors
	var combined []error
	for _, e := range errs {
		if e != nil {
			combined = append(combined, e)
		}
	}

	if len(combined) > 0 {
		return fmt.Errorf("scrape run completed with %d store error(s): %v", len(combined), combined)
	}
	return nil
}

// RunStore loads the store with the given ID and runs its worker.
func (r *Runner) RunStore(ctx context.Context, storeID int64) error {
	stores, err := r.repo.LoadActiveStoresWithRules(ctx, r.db)
	if err != nil {
		return fmt.Errorf("loading stores: %w", err)
	}

	for _, sw := range stores {
		if sw.Store.ID == storeID {
			return r.worker.Run(ctx, sw)
		}
	}

	return fmt.Errorf("store with id=%d not found or not active", storeID)
}
