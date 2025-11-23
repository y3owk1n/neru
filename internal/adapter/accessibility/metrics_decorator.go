package accessibility

import (
	"context"
	"image"
	"time"

	"github.com/y3owk1n/neru/internal/application/ports"
	"github.com/y3owk1n/neru/internal/domain/action"
	"github.com/y3owk1n/neru/internal/domain/element"
	"github.com/y3owk1n/neru/internal/infra/metrics"
)

// MetricsDecorator wraps an AccessibilityPort to collect metrics.
type MetricsDecorator struct {
	next      ports.AccessibilityPort
	collector *metrics.Collector
}

// NewMetricsDecorator creates a new MetricsDecorator.
func NewMetricsDecorator(next ports.AccessibilityPort, collector *metrics.Collector) *MetricsDecorator {
	return &MetricsDecorator{
		next:      next,
		collector: collector,
	}
}

func (d *MetricsDecorator) recordDuration(name string, start time.Time) {
	duration := time.Since(start).Seconds()
	d.collector.ObserveHistogram(name, duration, nil)
}

func (d *MetricsDecorator) recordError(name string, err error) {
	if err != nil {
		d.collector.IncCounter(name+"_errors", nil)
	}
}

// GetClickableElements implements ports.AccessibilityPort.
func (d *MetricsDecorator) GetClickableElements(ctx context.Context, filter ports.ElementFilter) ([]*element.Element, error) {
	defer d.recordDuration("accessibility_get_clickable_elements_duration", time.Now())
	elems, err := d.next.GetClickableElements(ctx, filter)
	d.recordError("accessibility_get_clickable_elements", err)
	if err == nil {
		d.collector.ObserveHistogram("accessibility_clickable_elements_count", float64(len(elems)), nil)
	}
	return elems, err
}

// GetScrollableElements implements ports.AccessibilityPort.
func (d *MetricsDecorator) GetScrollableElements(ctx context.Context) ([]*element.Element, error) {
	defer d.recordDuration("accessibility_get_scrollable_elements_duration", time.Now())
	elems, err := d.next.GetScrollableElements(ctx)
	d.recordError("accessibility_get_scrollable_elements", err)
	return elems, err
}

// PerformAction implements ports.AccessibilityPort.
func (d *MetricsDecorator) PerformAction(ctx context.Context, elem *element.Element, actionType action.Type) error {
	defer d.recordDuration("accessibility_perform_action_duration", time.Now())
	err := d.next.PerformAction(ctx, elem, actionType)
	d.recordError("accessibility_perform_action", err)
	return err
}

// PerformActionAtPoint implements ports.AccessibilityPort.
func (d *MetricsDecorator) PerformActionAtPoint(ctx context.Context, actionType action.Type, point image.Point) error {
	defer d.recordDuration("accessibility_perform_action_at_point_duration", time.Now())
	err := d.next.PerformActionAtPoint(ctx, actionType, point)
	d.recordError("accessibility_perform_action_at_point", err)
	return err
}

// Scroll implements ports.AccessibilityPort.
func (d *MetricsDecorator) Scroll(ctx context.Context, deltaX, deltaY int) error {
	defer d.recordDuration("accessibility_scroll_duration", time.Now())
	err := d.next.Scroll(ctx, deltaX, deltaY)
	d.recordError("accessibility_scroll", err)
	return err
}

// GetFocusedAppBundleID implements ports.AccessibilityPort.
func (d *MetricsDecorator) GetFocusedAppBundleID(ctx context.Context) (string, error) {
	return d.next.GetFocusedAppBundleID(ctx)
}

// IsAppExcluded implements ports.AccessibilityPort.
func (d *MetricsDecorator) IsAppExcluded(ctx context.Context, bundleID string) bool {
	return d.next.IsAppExcluded(ctx, bundleID)
}

// GetScreenBounds implements ports.AccessibilityPort.
func (d *MetricsDecorator) GetScreenBounds(ctx context.Context) (image.Rectangle, error) {
	return d.next.GetScreenBounds(ctx)
}

// MoveCursorToPoint implements ports.AccessibilityPort.
func (d *MetricsDecorator) MoveCursorToPoint(ctx context.Context, point image.Point) error {
	return d.next.MoveCursorToPoint(ctx, point)
}

// GetCursorPosition implements ports.AccessibilityPort.
func (d *MetricsDecorator) GetCursorPosition(ctx context.Context) (image.Point, error) {
	return d.next.GetCursorPosition(ctx)
}

// CheckPermissions implements ports.AccessibilityPort.
func (d *MetricsDecorator) CheckPermissions(ctx context.Context) error {
	return d.next.CheckPermissions(ctx)
}

// Health implements ports.AccessibilityPort.
func (d *MetricsDecorator) Health(ctx context.Context) error {
	return d.next.Health(ctx)
}
