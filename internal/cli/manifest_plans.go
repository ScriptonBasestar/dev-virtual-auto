package cli

import (
	"github.com/ScriptonBasestar/dva/internal/config"
	"github.com/ScriptonBasestar/dva/internal/lifecycle"
)

type ManifestPlan struct {
	Description     string              `json:"description,omitempty" yaml:"description,omitempty"`
	Environment     string              `json:"environment,omitempty" yaml:"environment,omitempty"`
	Site            string              `json:"site,omitempty" yaml:"site,omitempty"`
	Entries         []ManifestPlanEntry `json:"entries" yaml:"entries"`
	ResolutionError string              `json:"resolution_error,omitempty" yaml:"resolution_error,omitempty"`
}

type ManifestPlanEntry struct {
	Name      string   `json:"name" yaml:"name"`
	Runner    string   `json:"runner" yaml:"runner"`
	Order     int      `json:"order" yaml:"order"`
	DependsOn []string `json:"depends_on,omitempty" yaml:"depends_on,omitempty"`
	Services  []string `json:"services,omitempty" yaml:"services,omitempty"`
	Wave      int      `json:"wave" yaml:"wave"`
}

func buildManifestPlans(c *config.Config) map[string]ManifestPlan {
	if len(c.Plans) == 0 {
		return nil
	}

	plans := make(map[string]ManifestPlan, len(c.Plans))
	for _, name := range sortedKeys(c.Plans) {
		planConfig := c.Plans[name]
		if planConfig == nil {
			plans[name] = ManifestPlan{
				Entries:         []ManifestPlanEntry{},
				ResolutionError: "plan configuration is empty",
			}
			continue
		}

		plan := ManifestPlan{
			Description: planConfig.Description,
			Environment: planConfig.Environment,
			Site:        planConfig.Site,
			Entries:     make([]ManifestPlanEntry, 0, len(planConfig.Entries)),
		}
		resolved, err := lifecycle.ResolvePlan(c, name, nil)
		if err != nil {
			plan.ResolutionError = err.Error()
			plans[name] = plan
			continue
		}

		for _, entry := range resolved.Entries {
			plan.Entries = append(plan.Entries, ManifestPlanEntry{
				Name:      entry.Name,
				Runner:    entry.Runner,
				Order:     entry.Order,
				DependsOn: entry.DependsOn,
				Services:  entry.Services,
				Wave:      entry.Wave,
			})
		}
		plans[name] = plan
	}
	return plans
}
