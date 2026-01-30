package overlay

import (
	"context"
	"time"

	"github.com/y3owk1n/neru/internal/core/domain/hint"
	"github.com/y3owk1n/neru/internal/core/infra/metrics"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// MetricsDecorator wraps an OverlayPort to collect metrics.
type MetricsDecorator struct {
	next      ports.OverlayPort
	collector metrics.Collector
}

// NewMetricsDecorator creates a new MetricsDecorator.
func NewMetricsDecorator(
	next ports.OverlayPort,
	collector metrics.Collector,
) *MetricsDecorator {
	return &MetricsDecorator{
		next:      next,
		collector: collector,
	}
}

// Show shows the overlay.
func (d *MetricsDecorator) Show() {
	d.next.Show()
}

// ShowHints implements ports.OverlayPort.
func (d *MetricsDecorator) ShowHints(ctx context.Context, hints []*hint.Interface) error {
	defer d.recordDuration("overlay_show_hints_duration", time.Now())

	d.collector.ObserveHistogram("overlay_hints_count", float64(len(hints)), nil)

	return d.next.ShowHints(ctx, hints)
}

// ShowGrid implements ports.OverlayPort.
func (d *MetricsDecorator) ShowGrid(ctx context.Context) error {
	defer d.recordDuration("overlay_show_grid_duration", time.Now())

	return d.next.ShowGrid(ctx)
}

// DrawScrollIndicator implements ports.OverlayPort.
func (d *MetricsDecorator) DrawScrollIndicator(x, y int) {
	d.next.DrawScrollIndicator(x, y)
}

// Hide implements ports.OverlayPort.
func (d *MetricsDecorator) Hide(ctx context.Context) error {
	defer d.recordDuration("overlay_hide_duration", time.Now())

	return d.next.Hide(ctx)
}

// IsVisible implements ports.OverlayPort.
func (d *MetricsDecorator) IsVisible() bool {
	return d.next.IsVisible()
}

// Refresh implements ports.OverlayPort.
func (d *MetricsDecorator) Refresh(ctx context.Context) error {
	defer d.recordDuration("overlay_refresh_duration", time.Now())

	return d.next.Refresh(ctx)
}

// Health implements ports.OverlayPort.
func (d *MetricsDecorator) Health(ctx context.Context) error {
	return d.next.Health(ctx)
}

func (d *MetricsDecorator) recordDuration(name string, start time.Time) {
	duration := time.Since(start).Seconds()
	d.collector.ObserveHistogram(name, duration, nil)
}
