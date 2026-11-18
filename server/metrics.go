package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/dvcrn/mcp-server-revenuecat/revenuecat"
)

type getOverviewMetricsInput struct {
	Project  string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	Currency string `json:"currency,omitempty" jsonschema:"Optional ISO 4217 currency code (for example USD or EUR)."`
}

type getChartOptionsInput struct {
	Project   string `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	ChartName string `json:"chart_name" jsonschema:"Chart identifier, such as revenue, mrr, mrr_movement, arr, actives, actives_movement, actives_new, churn, trials, trials_movement, trials_new, customers_new, customers_active, conversion_to_paying, initial_conversion, trial_conversion, ltv_per_customer, ltv_per_paying_customer, or refund_rate."`
	Realtime  *bool  `json:"realtime,omitempty" jsonschema:"Whether to inspect options for realtime charts."`
}

// ChartFilter is a single chart filter entry serialized into the JSON array
// expected by RevenueCat's charts endpoint.
type ChartFilter struct {
	Name   string   `json:"name" jsonschema:"Filter dimension ID from get_chart_options (for example country or store)."`
	Values []string `json:"values" jsonschema:"Filter option values to match."`
}

type getChartDataInput struct {
	Project          string            `json:"project" jsonschema:"Configured project alias (see list_projects) or RevenueCat proj... ID."`
	ChartName        string            `json:"chart_name" jsonschema:"Chart identifier (for example revenue, mrr, arr, actives, churn, trials, conversion_to_paying, or refund_rate). Call get_chart_options to discover supported filters, segments, resolutions, and selectors."`
	StartDate        string            `json:"start_date,omitempty" jsonschema:"Start date in YYYY-MM-DD format."`
	EndDate          string            `json:"end_date,omitempty" jsonschema:"End date in YYYY-MM-DD format."`
	Resolution       string            `json:"resolution,omitempty" jsonschema:"Resolution ID from get_chart_options (for example day, week, month, or numeric resolution ID)."`
	Currency         string            `json:"currency,omitempty" jsonschema:"ISO 4217 currency code (for example USD or EUR)."`
	Segment          string            `json:"segment,omitempty" jsonschema:"Segment dimension ID from get_chart_options (for example country, store, or product_duration)."`
	Filters          []ChartFilter     `json:"filters,omitempty" jsonschema:"Chart filters; each entry specifies a filter name and list of values."`
	Selectors        map[string]string `json:"selectors,omitempty" jsonschema:"Chart-specific user selectors from get_chart_options (for example revenue_type)."`
	Aggregate        []string          `json:"aggregate,omitempty" jsonschema:"Summary aggregations to compute: average, total."`
	Realtime         *bool             `json:"realtime,omitempty" jsonschema:"Whether to query realtime chart data."`
	ExpandPeriods    *bool             `json:"expand_periods,omitempty" jsonschema:"Whether to expand multi-period chart rows."`
	LimitNumSegments *int              `json:"limit_num_segments,omitempty" jsonschema:"Maximum number of segments to return when segmenting."`
}

func registerMetricTools(r registry) {
	add(r, &mcp.Tool{
		Name:        "get_overview_metrics",
		Description: "Get current overview metrics for a RevenueCat project (active subscriptions, active trials, MRR, revenue, new customers, active users).",
		Annotations: readOnly(),
	}, getOverviewMetrics)

	add(r, &mcp.Tool{
		Name:        "get_chart_options",
		Description: "Get available resolutions, segments, filters, and user selectors for a RevenueCat chart (`chart_name`) before calling `get_chart_data`.",
		Annotations: readOnly(),
	}, getChartOptions)

	add(r, &mcp.Tool{
		Name:        "get_chart_data",
		Description: "Query time-series chart data and summary totals/averages for a RevenueCat project (`chart_name`, date range, resolution, currency, segment, filters, and selectors).",
		Annotations: readOnly(),
	}, getChartData)
}

func getOverviewMetrics(ctx context.Context, env *Env, in getOverviewMetricsInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	if err := q.SetString("currency", in.Currency); err != nil {
		return nil, nil, err
	}

	return getJSON(ctx, p, "/v2/projects/{project_id}/metrics/overview", []string{"projects", p.ProjectID, "metrics", "overview"}, q)
}

func getChartOptions(ctx context.Context, env *Env, in getChartOptionsInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	chartName, err := requireID("chart_name", in.ChartName)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	q.SetOptBool("realtime", in.Realtime)

	return getJSON(ctx, p, "/v2/projects/{project_id}/charts/{chart_name}/options", []string{"projects", p.ProjectID, "charts", chartName, "options"}, q)
}

func getChartData(ctx context.Context, env *Env, in getChartDataInput) (*mcp.CallToolResult, any, error) {
	p, err := env.resolveProject(in.Project)
	if err != nil {
		return nil, nil, err
	}

	chartName, err := requireID("chart_name", in.ChartName)
	if err != nil {
		return nil, nil, err
	}

	q := revenuecat.NewQuery()
	for _, kv := range [][2]string{
		{"start_date", in.StartDate},
		{"end_date", in.EndDate},
		{"resolution", in.Resolution},
		{"currency", in.Currency},
		{"segment", in.Segment},
	} {
		if err := q.SetString(kv[0], kv[1]); err != nil {
			return nil, nil, err
		}
	}

	if len(in.Filters) > 0 {
		for _, f := range in.Filters {
			if _, err := requireID("filters.name", f.Name); err != nil {
				return nil, nil, err
			}
			if len(f.Values) == 0 {
				return nil, nil, fmt.Errorf("filters.values for %q cannot be empty", f.Name)
			}
		}
		raw, err := json.Marshal(in.Filters)
		if err != nil {
			return nil, nil, fmt.Errorf("encode filters: %w", err)
		}
		if err := q.SetString("filters", string(raw)); err != nil {
			return nil, nil, err
		}
	}

	if len(in.Selectors) > 0 {
		raw, err := json.Marshal(in.Selectors)
		if err != nil {
			return nil, nil, fmt.Errorf("encode selectors: %w", err)
		}
		if err := q.SetString("selectors", string(raw)); err != nil {
			return nil, nil, err
		}
	}

	if err := q.AddStrings("aggregate", in.Aggregate); err != nil {
		return nil, nil, err
	}

	q.SetOptBool("realtime", in.Realtime)
	q.SetOptBool("expand_periods", in.ExpandPeriods)
	if in.LimitNumSegments != nil {
		if *in.LimitNumSegments < 1 {
			return nil, nil, fmt.Errorf("limit_num_segments must be at least 1, got %d", *in.LimitNumSegments)
		}
		q.SetOptInt("limit_num_segments", in.LimitNumSegments)
	}

	return getJSON(ctx, p, "/v2/projects/{project_id}/charts/{chart_name}", []string{"projects", p.ProjectID, "charts", chartName}, q)
}
