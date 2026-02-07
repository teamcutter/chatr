package resolver

import (
	"context"
	"fmt"

	"github.com/teamcutter/chatr/internal/domain"
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
	var result []ResolvedPackage
	visiting := make(map[string]bool)
	visited := make(map[string]bool)

	if err := r.resolve(ctx, name, false, visiting, visited, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *Resolver) resolve(ctx context.Context, name string, isDep bool, visiting, visited map[string]bool, result *[]ResolvedPackage,
) error {
	if visited[name] {
		return nil
	}

	if visiting[name] {
		return fmt.Errorf("dependency cycle detected: %s", name)
	}

	alreadyInstalled := false
	if isDep {
		if installed, _, _ := r.state.IsInstalled(name); installed {
			alreadyInstalled = true
		}
	}

	visiting[name] = true

	formula, err := r.registry.Get(ctx, name)
	if err != nil {
		return fmt.Errorf("resolving %s: %w", name, err)
	}

	for _, dep := range formula.Dependencies {
		if err := r.resolve(ctx, dep, true, visiting, visited, result); err != nil {
			return err
		}
	}

	delete(visiting, name)
	visited[name] = true

	*result = append(*result, ResolvedPackage{
		Formula:          *formula,
		IsDep:            isDep,
		AlreadyInstalled: alreadyInstalled,
	})

	return nil
}
