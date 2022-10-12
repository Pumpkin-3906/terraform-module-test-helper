package terraform_module_test_helper

import (
	"github.com/ahmetb/go-linq/v3"
	"github.com/hashicorp/terraform-config-inspect/tfconfig"
	"github.com/r3labs/diff/v3"
)

type ChangeCategory = string

const (
	Variable ChangeCategory = "Variables"
	Output   ChangeCategory = "Outputs"
)

type BreakingChange struct {
	diff.Change
	Category  ChangeCategory `json:"category"`
	Name      *string        `json:"name"`
	Attribute *string        `json:"attribute"`
}

func BreakingChanges(m1 *tfconfig.Module, m2 *tfconfig.Module) ([]BreakingChange, error) {
	sanitizeModule(m1)
	sanitizeModule(m2)
	changelog, err := diff.Diff(m1, m2)
	if err != nil {
		return nil, err
	}
	return filterBreakingChanges(convert(changelog)), nil
}

func sanitizeModule(m *tfconfig.Module) {
	m.Path = ""
	for _, v := range m.Variables {
		v.Pos = *new(tfconfig.SourcePos)
	}
	for _, r := range m.ManagedResources {
		r.Pos = *new(tfconfig.SourcePos)
	}
	for _, r := range m.DataResources {
		r.Pos = *new(tfconfig.SourcePos)
	}
	for _, o := range m.Outputs {
		o.Pos = *new(tfconfig.SourcePos)
	}
}

func convert(cl diff.Changelog) (r []BreakingChange) {
	linq.From(cl).Select(func(i interface{}) interface{} {
		c := i.(diff.Change)
		var name, attribute *string
		if len(c.Path) > 1 {
			name = &c.Path[1]
		}
		if len(c.Path) > 2 {
			attribute = &c.Path[2]
		}
		return BreakingChange{
			Change: diff.Change{
				Type: c.Type,
				Path: c.Path,
				From: c.From,
				To:   c.To,
			},
			Category:  c.Path[0],
			Name:      name,
			Attribute: attribute,
		}
	}).ToSlice(&r)
	return
}

func filterBreakingChanges(cl []BreakingChange) []BreakingChange {
	var r []BreakingChange
	variables := linq.From(cl).Where(func(i interface{}) bool {
		return i.(BreakingChange).Category == Variable
	})
	newVariables := variables.Where(isNewVariable)
	requiredNewVariables := groupByName(newVariables).Where(noDefaultValue)
	requiredNewVariables.Select(recordForName).ToSlice(&r)
	return r
}

func recordForName(g interface{}) interface{} {
	return linq.From(g.(linq.Group).Group).FirstWith(func(i interface{}) bool {
		return i.(BreakingChange).Attribute != nil && *i.(BreakingChange).Attribute == "Name"
	})
}

func groupByName(newVariables linq.Query) linq.Query {
	return newVariables.GroupBy(func(i interface{}) interface{} {
		return *i.(BreakingChange).Name
	}, func(i interface{}) interface{} {
		return i
	})
}

func noDefaultValue(g interface{}) bool {
	return linq.From(g.(linq.Group).Group).All(func(i interface{}) bool {
		return i.(BreakingChange).Attribute == nil || *i.(BreakingChange).Attribute != "default"
	})
}

func isNewVariable(i interface{}) bool {
	return i.(BreakingChange).Type == "create"
}