package resolver

import (
	"context"
	"fmt"
	"sync"

	"github.com/teamcutter/chatr/internal/domain"
	"golang.org/x/sync/errgroup"
)

type Resolver struct {
	registry domain.Registry
	state    domain.State
}

type ResolvedPackage struct {
	Formula          domain.Formula
	IsDep            bool
	AlreadyInstalled bool
}

func New(registry domain.Registry, state domain.State) *Resolver {
	return &Resolver{
		registry: registry,
		state:    state,
	}
}

func (r *Resolver) Resolve(ctx context.Context, name string) ([]ResolvedPackage, error) {
	var mu sync.Mutex
	fetched := make(map[string]*domain.Formula)

	if err := r.fetchAll(ctx, name, fetched, &mu); err != nil {
		return nil, err
	}

	var result []ResolvedPackage
	visited := make(map[string]bool)
	r.buildResult(name, false, fetched, visited, &result)

	return result, nil
}

func (r *Resolver) fetchAll(ctx context.Context, name string, fetched map[string]*domain.Formula, mu *sync.Mutex) error {
	mu.Lock()
	if _, exists := fetched[name]; exists {
		mu.Unlock()
		return nil
	}
	fetched[name] = nil
	mu.Unlock()

	formula, err := r.registry.Get(ctx, name)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", name, err)
	}

	mu.Lock()
	fetched[name] = formula
	deps := formula.Dependencies
	mu.Unlock()

	g, ctx := errgroup.WithContext(ctx)
	for _, dep := range deps {
		g.Go(func() error {
			return r.fetchAll(ctx, dep, fetched, mu)
		})
	}

	return g.Wait()
}

func (r *Resolver) buildResult(name string, isDep bool, fetched map[string]*domain.Formula, visited map[string]bool, result *[]ResolvedPackage) {
	if visited[name] {
		return
	}
	visited[name] = true

	formula := fetched[name]
	for _, dep := range formula.Dependencies {
		r.buildResult(dep, true, fetched, visited, result)
	}

	alreadyInstalled := false
	if isDep {
		if installed, _, _ := r.state.IsInstalled(name); installed {
			alreadyInstalled = true
		}
	}

	*result = append(*result, ResolvedPackage{
		Formula:          *formula,
		IsDep:            isDep,
		AlreadyInstalled: alreadyInstalled,
	})
}
