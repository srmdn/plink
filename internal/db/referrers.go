package db

import (
	"net/url"
	"sort"
	"strings"
)

type referrerAggregate struct {
	Source  string
	Clicks  int64
	LinkIDs map[int64]struct{}
	Details map[string]int64
}

// normalizeReferrer reduces a recorded URL to the host that sent the click.
// Direct traffic keeps a stable sentinel so it remains separate from malformed
// or non-HTTP referrer values.
func normalizeReferrer(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "direct") {
		return "direct"
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" {
		return raw
	}

	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if host == "" {
		return raw
	}
	return host
}

func addReferrer(groups map[string]*referrerAggregate, raw string, linkID int64) {
	raw = strings.TrimSpace(raw)
	source := normalizeReferrer(raw)
	aggregate := groups[source]
	if aggregate == nil {
		aggregate = &referrerAggregate{
			Source:  source,
			LinkIDs: make(map[int64]struct{}),
			Details: make(map[string]int64),
		}
		groups[source] = aggregate
	}
	aggregate.Clicks++
	aggregate.LinkIDs[linkID] = struct{}{}
	if raw != "" && source != "direct" {
		aggregate.Details[raw]++
	}
}

func sortedReferrerDetails(details map[string]int64) []ReferrerDetail {
	result := make([]ReferrerDetail, 0, len(details))
	for source, clicks := range details {
		result = append(result, ReferrerDetail{Source: source, Clicks: clicks})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Clicks != result[j].Clicks {
			return result[i].Clicks > result[j].Clicks
		}
		return result[i].Source < result[j].Source
	})
	return result
}

func sortedReferrerAggregates(groups map[string]*referrerAggregate) []*referrerAggregate {
	result := make([]*referrerAggregate, 0, len(groups))
	for _, aggregate := range groups {
		result = append(result, aggregate)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Clicks != result[j].Clicks {
			return result[i].Clicks > result[j].Clicks
		}
		return result[i].Source < result[j].Source
	})
	return result
}

func limitAggregates(groups []*referrerAggregate, limit int) []*referrerAggregate {
	if limit > 0 && len(groups) > limit {
		return groups[:limit]
	}
	return groups
}

func buildReferrers(groups map[string]*referrerAggregate, limit int) []Referrer {
	aggregates := limitAggregates(sortedReferrerAggregates(groups), limit)
	result := make([]Referrer, 0, len(aggregates))
	for _, aggregate := range aggregates {
		result = append(result, Referrer{
			Source:  aggregate.Source,
			Clicks:  aggregate.Clicks,
			Details: sortedReferrerDetails(aggregate.Details),
		})
	}
	return result
}

func buildSourceSummaries(groups map[string]*referrerAggregate, limit int) []SourceSummary {
	aggregates := limitAggregates(sortedReferrerAggregates(groups), limit)
	result := make([]SourceSummary, 0, len(aggregates))
	for _, aggregate := range aggregates {
		result = append(result, SourceSummary{
			Source:    aggregate.Source,
			Clicks:    aggregate.Clicks,
			LinkCount: int64(len(aggregate.LinkIDs)),
			Details:   sortedReferrerDetails(aggregate.Details),
		})
	}
	return result
}
