package agent

import "errors"

const DefaultReconcileLimit = 1024

type EventSource interface {
	ListReady(limit int) ([]Event, error)
}

type Reconciler struct {
	Source EventSource
	Runner Runner
	Limit  int
}

type ReconcileReport struct {
	Processed int
	Created   int
	Existing  int
	Skipped   int
	Failed    int
	Items     []ReconcileItem
}

type ReconcileItem struct {
	Event      Event
	EvidenceID string
	Created    bool
	Skipped    bool
	Error      string
}

var ErrReconcilerNotConfigured = errors.New("paperless reconciler is not configured")

// Run replays the current ready Paperless candidates through the same idempotent
// ingestion path as webhook handling. Item failures are reported without hiding
// successful entries in the same reconciliation pass.
func (r Reconciler) Run() (ReconcileReport, error) {
	if r.Source == nil {
		return ReconcileReport{}, ErrReconcilerNotConfigured
	}
	limit := r.Limit
	if limit == 0 {
		limit = DefaultReconcileLimit
	}
	if limit < 0 {
		return ReconcileReport{}, ErrReconcilerNotConfigured
	}
	events, err := r.Source.ListReady(limit)
	if err != nil {
		return ReconcileReport{}, err
	}
	report := ReconcileReport{Items: make([]ReconcileItem, 0, len(events))}
	for _, event := range events {
		result, err := r.Runner.Handle(event)
		item := ReconcileItem{Event: event}
		if err != nil {
			item.Error = err.Error()
			report.Failed++
		} else {
			item.EvidenceID = result.EvidenceID
			item.Created = result.Created
			item.Skipped = result.Skipped
			switch {
			case result.Skipped:
				report.Skipped++
			case result.Created:
				report.Created++
			default:
				report.Existing++
			}
		}
		report.Processed++
		report.Items = append(report.Items, item)
	}
	return report, nil
}
