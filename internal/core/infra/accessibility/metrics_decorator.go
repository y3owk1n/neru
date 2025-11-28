package accessibility

import (
	"context"
	"image"
	"time"

	"github.com/y3owk1n/neru/internal/core/domain/action"
	"github.com/y3owk1n/neru/internal/core/domain/element"
	"github.com/y3owk1n/neru/internal/core/infra/metrics"
	"github.com/y3owk1n/neru/internal/core/ports"
)

// MetricsDecorator wraps an AccessibilityPort to collect metrics.
type MetricsDecorator struct {
	next      ports.AccessibilityPort
	collector metrics.Collector
}

// NewMetricsDecorator creates a new MetricsDecorator.
func NewMetricsDecorator(
	next ports.AccessibilityPort,
	collector metrics.Collector,
) *MetricsDecorator {
	return &MetricsDecorator{
		next:      next,
		collector: collector,
	}
}

// ClickableElements implements ports.AccessibilityPort.
func (d *MetricsDecorator) ClickableElements(
	ctx context.Context,
	filter ports.ElementFilter,
) ([]*element.Element, error) {
	defer d.recordDuration("accessibility_get_clickable_elements_duration", time.Now())

	elements, elementsErr := d.next.ClickableElements(ctx, filter)
	d.recordError("accessibility_get_clickable_elements", elementsErr)

	if elementsErr == nil {
		d.collector.ObserveHistogram(
			"accessibility_clickable_elements_count",
			float64(len(elements)),
			nil,
		)
	}

	return elements, elementsErr
}

// PerformAction implements ports.AccessibilityPort.
func (d *MetricsDecorator) PerformAction(
	ctx context.Context,
	element *element.Element,
	actionType action.Type,
) error {
	defer d.recordDuration("accessibility_perform_action_duration", time.Now())

	performActionErr := d.next.PerformAction(ctx, element, actionType)
	d.recordError("accessibility_perform_action", performActionErr)

	return performActionErr
}

// PerformActionAtPoint implements ports.AccessibilityPort.
func (d *MetricsDecorator) PerformActionAtPoint(
	ctx context.Context,
	actionType action.Type,
	point image.Point,
) error {
	defer d.recordDuration("accessibility_perform_action_at_point_duration", time.Now())

	performActionErr := d.next.PerformActionAtPoint(ctx, actionType, point)
	d.recordError("accessibility_perform_action_at_point", performActionErr)

	return performActionErr
}

// Scroll implements ports.AccessibilityPort.
func (d *MetricsDecorator) Scroll(ctx context.Context, deltaX, deltaY int) error {
	defer d.recordDuration("accessibility_scroll_duration", time.Now())

	scrollErr := d.next.Scroll(ctx, deltaX, deltaY)
	d.recordError("accessibility_scroll", scrollErr)

	return scrollErr
}

// FocusedAppBundleID implements ports.AccessibilityPort.
func (d *MetricsDecorator) FocusedAppBundleID(ctx context.Context) (string, error) {
	return d.next.FocusedAppBundleID(ctx)
}

// IsAppExcluded implements ports.AccessibilityPort.
func (d *MetricsDecorator) IsAppExcluded(ctx context.Context, bundleID string) bool {
	return d.next.IsAppExcluded(ctx, bundleID)
}

// ScreenBounds implements ports.AccessibilityPort.
func (d *MetricsDecorator) ScreenBounds(ctx context.Context) (image.Rectangle, error) {
	return d.next.ScreenBounds(ctx)
}

// MoveCursorToPoint implements ports.AccessibilityPort.
func (d *MetricsDecorator) MoveCursorToPoint(ctx context.Context, point image.Point) error {
	return d.next.MoveCursorToPoint(ctx, point)
}

// CursorPosition implements ports.AccessibilityPort.
func (d *MetricsDecorator) CursorPosition(ctx context.Context) (image.Point, error) {
	return d.next.CursorPosition(ctx)
}

// CheckPermissions implements ports.AccessibilityPort.
func (d *MetricsDecorator) CheckPermissions(ctx context.Context) error {
	return d.next.CheckPermissions(ctx)
}

// Health implements ports.AccessibilityPort.
func (d *MetricsDecorator) Health(ctx context.Context) error {
	return d.next.Health(ctx)
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
