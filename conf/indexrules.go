package conf

import (
	"fmt"
	"strings"
	"time"

	"github.com/alyu/configparser"
)

// IndexRules holds the aggregation definitions
type IndexRules struct {
	Rules   []IndexRule
	Default IndexRule
}

type IndexRule struct {
	Name     string
	Pattern  string
	MaxStale time.Duration
}

// NewIndexRules create instance of IndexRules
// it has a default catchall that doesn't prune
func NewIndexRules() IndexRules {
	return IndexRules{
		Default: IndexRule{
			Name: "default",
		},
	}
}

// ReadIndexRules returns the defined aggregations from a index-rules.conf file
// and adds the default
func ReadIndexRules(file string) (IndexRules, error) {
	config, err := configparser.Read(file)
	if err != nil {
		return IndexRules{}, err
	}
	sections, err := config.AllSections()
	if err != nil {
		return IndexRules{}, err
	}

	result := NewIndexRules()

	for _, s := range sections {
		item := IndexRule{}
		item.Name = strings.Trim(strings.SplitN(s.String(), "\n", 2)[0], " []")
		if item.Name == "" || strings.HasPrefix(item.Name, "#") {
			continue
		}

		item.Pattern = s.ValueOf("prefix")

		item.MaxStale, err = time.ParseDuration(s.ValueOf("maxStale"))
		if err != nil {
			return IndexRules{}, fmt.Errorf("[%s]: failed to parse max-stale %q: %s", item.Name, s.ValueOf("max-stale"), err.Error())
		}

		result.Rules = append(result.Rules, item)
	}

	return result, nil
}

// Match returns the correct aggregation setting for the given metric
// it can always find a valid setting, because there's a default catch all
// also returns the index of the setting, to efficiently reference it
func (a IndexRules) Match(metric string) (uint16, IndexRule) {
	for i, s := range a.Rules {
		if strings.HasPrefix(metric, s.Pattern) {
			return uint16(i), s
		}
	}
	return uint16(len(a.Rules)), a.Default
}

// Get returns the aggregation setting corresponding to the given index
func (a IndexRules) Get(i uint16) IndexRule {
	if i+1 > uint16(len(a.Rules)) {
		return a.Default
	}
	return a.Rules[i]
}

// Prunable returns whether there's any entries that require pruning
func (a IndexRules) Prunable() bool {
	for _, r := range a.Rules {
		if r.MaxStale > 0 {
			return true
		}
	}
	return (a.Default.MaxStale > 0)
}

type IndexCheck struct {
	Keep   bool
	Cutoff int64
}

// Checks returns a set of checks corresponding to a given timestamp and the set of all rules
func (a IndexRules) Checks(now time.Time) []IndexCheck {
	out := make([]IndexCheck, len(a.Rules)+1)
	for i, r := range a.Rules {
		out[i] = IndexCheck{
			Keep:   r.MaxStale == 0,
			Cutoff: int64(now.Add(r.MaxStale * -1).Unix()),
		}
	}
	out[len(a.Rules)] = IndexCheck{
		Keep:   a.Default.MaxStale == 0,
		Cutoff: int64(now.Add(a.Default.MaxStale * -1).Unix()),
	}
	return out
}
