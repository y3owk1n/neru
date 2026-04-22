//go:build linux && cgo

package overlay

/*
#cgo linux pkg-config: x11 xrender xfixes xext cairo
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/Xatom.h>
#include <X11/extensions/Xfixes.h>
#include <X11/extensions/shape.h>
#include <cairo/cairo.h>
#include <cairo/cairo-xlib.h>
#include <stdlib.h>

typedef struct {
	Display *display;
	int screen;
	Window root;
	Window window;
	Visual *visual;
	Colormap colormap;
	cairo_surface_t *surface;
	cairo_t *cr;
	int width;
	int height;
} NeruX11Overlay;

static Visual* neru_x11_argb_visual(Display *display, int screen) {
	XVisualInfo vinfo;
	if (XMatchVisualInfo(display, screen, 32, TrueColor, &vinfo)) {
		return vinfo.visual;
	}
	return DefaultVisual(display, screen);
}

static NeruX11Overlay* neru_x11_overlay_new(void) {
	Display *display = XOpenDisplay(NULL);
	if (display == NULL) {
		return NULL;
	}

	NeruX11Overlay *overlay = calloc(1, sizeof(NeruX11Overlay));
	overlay->display = display;
	overlay->screen = DefaultScreen(display);
	overlay->root = RootWindow(display, overlay->screen);
	overlay->visual = neru_x11_argb_visual(display, overlay->screen);
	overlay->width = DisplayWidth(display, overlay->screen);
	overlay->height = DisplayHeight(display, overlay->screen);
	overlay->colormap = XCreateColormap(display, overlay->root, overlay->visual, AllocNone);

	XSetWindowAttributes attrs;
	attrs.override_redirect = True;
	attrs.colormap = overlay->colormap;
	attrs.background_pixel = 0;
	attrs.border_pixel = 0;
	attrs.event_mask = ExposureMask;

	overlay->window = XCreateWindow(
		display,
		overlay->root,
		0, 0,
		overlay->width,
		overlay->height,
		0,
		32,
		InputOutput,
		overlay->visual,
		CWOverrideRedirect | CWColormap | CWBackPixel | CWBorderPixel | CWEventMask,
		&attrs
	);

	Atom dock = XInternAtom(display, "_NET_WM_WINDOW_TYPE_DOCK", False);
	Atom window_type = XInternAtom(display, "_NET_WM_WINDOW_TYPE", False);
	XChangeProperty(display, overlay->window, window_type, XA_ATOM, 32, PropModeReplace, (unsigned char *)&dock, 1);

	Atom state = XInternAtom(display, "_NET_WM_STATE", False);
	Atom above = XInternAtom(display, "_NET_WM_STATE_ABOVE", False);
	XChangeProperty(display, overlay->window, state, XA_ATOM, 32, PropModeReplace, (unsigned char *)&above, 1);

	XserverRegion region = XFixesCreateRegion(display, NULL, 0);
	XFixesSetWindowShapeRegion(display, overlay->window, ShapeInput, 0, 0, region);
	XFixesDestroyRegion(display, region);

	overlay->surface = cairo_xlib_surface_create(display, overlay->window, overlay->visual, overlay->width, overlay->height);
	overlay->cr = cairo_create(overlay->surface);
	XFlush(display);

	return overlay;
}

static void neru_x11_overlay_destroy(NeruX11Overlay *overlay) {
	if (overlay == NULL) {
		return;
	}
	if (overlay->cr != NULL) {
		cairo_destroy(overlay->cr);
	}
	if (overlay->surface != NULL) {
		cairo_surface_destroy(overlay->surface);
	}
	if (overlay->window != 0) {
		XDestroyWindow(overlay->display, overlay->window);
	}
	if (overlay->colormap != 0) {
		XFreeColormap(overlay->display, overlay->colormap);
	}
	if (overlay->display != NULL) {
		XCloseDisplay(overlay->display);
	}
	free(overlay);
}

static void neru_x11_overlay_show(NeruX11Overlay *overlay) {
	XMapRaised(overlay->display, overlay->window);
	XFlush(overlay->display);
}

static void neru_x11_overlay_hide(NeruX11Overlay *overlay) {
	XUnmapWindow(overlay->display, overlay->window);
	XFlush(overlay->display);
}

static void neru_x11_overlay_clear(NeruX11Overlay *overlay) {
	cairo_save(overlay->cr);
	cairo_set_operator(overlay->cr, CAIRO_OPERATOR_CLEAR);
	cairo_paint(overlay->cr);
	cairo_restore(overlay->cr);
	cairo_surface_flush(overlay->surface);
	XClearWindow(overlay->display, overlay->window);
	XFlush(overlay->display);
}

static void neru_x11_overlay_clear_rect(NeruX11Overlay *overlay, int x, int y, int width, int height) {
	if (overlay == NULL || width <= 0 || height <= 0) {
		return;
	}

	cairo_save(overlay->cr);
	cairo_set_operator(overlay->cr, CAIRO_OPERATOR_CLEAR);
	cairo_rectangle(overlay->cr, x, y, width, height);
	cairo_fill(overlay->cr);
	cairo_restore(overlay->cr);
	cairo_surface_flush(overlay->surface);
	XClearArea(overlay->display, overlay->window, x, y, (unsigned int)width, (unsigned int)height, False);
	XFlush(overlay->display);
}

static void neru_x11_overlay_resize(NeruX11Overlay *overlay) {
	int width = DisplayWidth(overlay->display, overlay->screen);
	int height = DisplayHeight(overlay->display, overlay->screen);
	if (width == overlay->width && height == overlay->height) {
		return;
	}
	overlay->width = width;
	overlay->height = height;
	XResizeWindow(overlay->display, overlay->window, width, height);
	cairo_xlib_surface_set_size(overlay->surface, width, height);
	XFlush(overlay->display);
}

static void neru_x11_overlay_color(cairo_t *cr, unsigned int color) {
	double a = ((color >> 24) & 0xFF) / 255.0;
	double r = ((color >> 16) & 0xFF) / 255.0;
	double g = ((color >> 8) & 0xFF) / 255.0;
	double b = (color & 0xFF) / 255.0;
	cairo_set_source_rgba(cr, r, g, b, a);
}

static void neru_x11_overlay_rect(
	NeruX11Overlay *overlay,
	double x, double y, double width, double height,
	unsigned int fill, unsigned int stroke, double stroke_width
) {
	cairo_t *cr = overlay->cr;
	cairo_save(cr);
	cairo_rectangle(cr, x, y, width, height);
	neru_x11_overlay_color(cr, fill);
	cairo_fill_preserve(cr);
	neru_x11_overlay_color(cr, stroke);
	cairo_set_line_width(cr, stroke_width);
	cairo_stroke(cr);
	cairo_restore(cr);
}

static void neru_x11_overlay_text(
	NeruX11Overlay *overlay,
	const char *text,
	const char *font_family,
	double x, double y,
	double font_size,
	unsigned int color
) {
	cairo_t *cr = overlay->cr;
	cairo_text_extents_t extents;
	cairo_save(cr);
	cairo_select_font_face(
		cr,
		font_family && font_family[0] ? font_family : "Sans",
		CAIRO_FONT_SLANT_NORMAL,
		CAIRO_FONT_WEIGHT_BOLD
	);
	cairo_set_font_size(cr, font_size);
	cairo_text_extents(cr, text, &extents);
	neru_x11_overlay_color(cr, color);
	cairo_move_to(cr, x - (extents.width / 2.0) - extents.x_bearing, y - (extents.height / 2.0) - extents.y_bearing);
	cairo_show_text(cr, text);
	cairo_restore(cr);
}

static void neru_x11_overlay_flush(NeruX11Overlay *overlay) {
	cairo_surface_flush(overlay->surface);
	XFlush(overlay->display);
}
*/
import "C"

import (
	"image"
	"strings"
	"unsafe"

	"go.uber.org/zap"

	gridcomponent "github.com/y3owk1n/neru/internal/app/components/grid"
	hintscomponent "github.com/y3owk1n/neru/internal/app/components/hints"
	recursivegridcomponent "github.com/y3owk1n/neru/internal/app/components/recursivegrid"
	domainGrid "github.com/y3owk1n/neru/internal/core/domain/grid"
)

type x11Overlay struct {
	raw            *C.NeruX11Overlay
	logger         *zap.Logger
	currentPrefix  string
	hideUnmatched  bool
	currentSubgrid *domainGrid.Cell
	sublayerKeys   string
	cachedGrid     *domainGrid.Grid
	cachedStyle    gridcomponent.Style
}

func newX11Overlay(logger *zap.Logger) *x11Overlay {
	raw := C.neru_x11_overlay_new()
	if raw == nil {
		return nil
	}

	return &x11Overlay{raw: raw, logger: logger}
}

func (o *x11Overlay) Healthy() bool {
	return o != nil && o.raw != nil
}

func (o *x11Overlay) WindowPtr() unsafe.Pointer {
	if o == nil {
		return nil
	}

	return unsafe.Pointer(o.raw)
}

func (o *x11Overlay) Show() {
	if o != nil && o.raw != nil {
		C.neru_x11_overlay_show(o.raw)
	}
}

func (o *x11Overlay) Hide() {
	if o != nil && o.raw != nil {
		C.neru_x11_overlay_hide(o.raw)
	}
}

func (o *x11Overlay) Clear() {
	if o != nil && o.raw != nil {
		C.neru_x11_overlay_clear(o.raw)
	}
}

func (o *x11Overlay) ClearRect(rect image.Rectangle) {
	if o != nil && o.raw != nil && !rect.Empty() {
		C.neru_x11_overlay_clear_rect(
			o.raw,
			C.int(rect.Min.X),
			C.int(rect.Min.Y),
			C.int(rect.Dx()),
			C.int(rect.Dy()),
		)
	}
}

func (o *x11Overlay) Resize() {
	if o != nil && o.raw != nil {
		C.neru_x11_overlay_resize(o.raw)
	}
}

func (o *x11Overlay) Destroy() {
	if o != nil && o.raw != nil {
		C.neru_x11_overlay_destroy(o.raw)
		o.raw = nil
	}
}

func (o *x11Overlay) UpdateGridMatches(prefix string) {
	o.currentPrefix = strings.ToUpper(prefix)
	o.redrawGrid()
}

func (o *x11Overlay) ShowSubgrid(cell *domainGrid.Cell, _ gridcomponent.Style) {
	if o == nil || o.raw == nil || cell == nil {
		return
	}

	o.currentSubgrid = cell
	o.Clear()
	o.drawSubgrid(cell.Bounds(), o.cachedStyle)
	C.neru_x11_overlay_flush(o.raw)
}

func (o *x11Overlay) SetHideUnmatched(hide bool) {
	o.hideUnmatched = hide
}

func (o *x11Overlay) DrawGrid(g *domainGrid.Grid, input string, style gridcomponent.Style) {
	if o == nil || o.raw == nil || g == nil {
		return
	}
	o.cachedGrid = g
	o.cachedStyle = style
	o.currentPrefix = strings.ToUpper(input)
	o.currentSubgrid = nil

	o.redrawGrid()
}

func (o *x11Overlay) DrawRecursiveGrid(
	bounds image.Rectangle,
	_ int,
	keys string,
	gridCols int,
	gridRows int,
	style recursivegridcomponent.Style,
	virtualPointer recursivegridcomponent.VirtualPointerState,
) {
	if o == nil || o.raw == nil || bounds.Empty() || gridCols <= 0 || gridRows <= 0 {
		return
	}
	o.Clear()

	keyRunes := []rune(strings.ToUpper(keys))
	cellWidth := bounds.Dx() / gridCols
	cellHeight := bounds.Dy() / gridRows
	index := 0
	for row := range gridRows {
		for col := range gridCols {
			cell := image.Rect(
				bounds.Min.X+col*cellWidth,
				bounds.Min.Y+row*cellHeight,
				bounds.Min.X+(col+1)*cellWidth,
				bounds.Min.Y+(row+1)*cellHeight,
			)
			if col == gridCols-1 {
				cell.Max.X = bounds.Max.X
			}
			if row == gridRows-1 {
				cell.Max.Y = bounds.Max.Y
			}

			fill := style.HighlightColor
			if fill == 0 {
				fill = subgridCellBackground
			}

			o.drawRect(cell, fill, style.LineColor, style.LineWidth)
			if index < len(keyRunes) {
				label := string(keyRunes[index])
				if style.LabelBackground {
					o.drawLabelBackground(label, cell, style)
				}
				o.drawTextCentered(
					label,
					cell,
					style.LabelFontName,
					style.LabelFontSize,
					style.LabelFontColor,
				)

				if shouldShowSubKeyPreview(cell, style) {
					o.drawSubKeyPreview(label, cell, style)
				}
			}
			index++
		}
	}

	if virtualPointer.Visible {
		vpBounds := image.Rect(
			virtualPointer.Position.X-virtualPointer.Size/2,
			virtualPointer.Position.Y-virtualPointer.Size/2,
			virtualPointer.Position.X+virtualPointer.Size/2,
			virtualPointer.Position.Y+virtualPointer.Size/2,
		)
		o.drawRect(
			vpBounds,
			parseHexColor(virtualPointer.FillColor),
			style.LineColor,
			subgridLineWidth,
		)
	}

	C.neru_x11_overlay_flush(o.raw)
}

func (o *x11Overlay) DrawBadge(
	posX,
	posY int,
	text string,
	colors overlayColors,
	style overlayBadgeStyle,
) {
	if o == nil || o.raw == nil || text == "" {
		return
	}

	fontSize := style.fontSize
	if fontSize <= 0 {
		fontSize = 14
	}

	rect := badgeBounds(posX, posY, text, style)

	o.drawRect(rect, colors.background, colors.border, max(style.borderWidth, 1))
	o.drawTextCentered(text, rect, style.fontFamily, fontSize, colors.text)
	C.neru_x11_overlay_flush(o.raw)
}

func (o *x11Overlay) DrawHints(hintsSlice []*hintscomponent.Hint, style hintscomponent.StyleMode) {
	if o == nil || o.raw == nil {
		return
	}

	o.Clear()
	for _, hint := range hintsSlice {
		bounds := image.Rect(
			hint.Position().X,
			hint.Position().Y,
			hint.Position().X+hint.Size().X,
			hint.Position().Y+hint.Size().Y,
		)

		textColor := style.TextColor()
		if hint.MatchedPrefix() != "" {
			textColor = style.MatchedTextColor()
		}

		o.drawRect(
			bounds,
			parseHexColor(style.BackgroundColor()),
			parseHexColor(style.BorderColor()),
			float64(max(style.BorderWidth(), 0)),
		)
		o.drawTextCentered(
			hint.Label(),
			bounds,
			style.FontFamily(),
			float64(max(style.FontSize(), 1)),
			parseHexColor(textColor),
		)
	}

	C.neru_x11_overlay_flush(o.raw)
}

func (o *x11Overlay) redrawGrid() {
	if o == nil || o.raw == nil || o.cachedGrid == nil {
		return
	}
	o.Clear()

	style := o.cachedStyle
	prefix := o.currentPrefix

	for _, cell := range o.cachedGrid.AllCells() {
		label := strings.ToUpper(cell.Coordinate())
		matched := strings.HasPrefix(label, prefix)
		if o.hideUnmatched && prefix != "" && !matched {
			continue
		}

		fill := style.BackgroundColor
		text := style.LabelFontColor
		border := style.LineColor
		if matched && prefix != "" {
			fill = style.MatchedBackgroundColor
			text = style.MatchedTextColor
			border = style.MatchedBorderColor
		}
		o.drawRect(cell.Bounds(), fill, border, style.LineWidth)
		o.drawTextCentered(label, cell.Bounds(), style.LabelFontName, style.LabelFontSize, text)
	}

	if o.currentSubgrid != nil {
		o.drawSubgrid(o.currentSubgrid.Bounds(), style)
	}
	C.neru_x11_overlay_flush(o.raw)
}

func (o *x11Overlay) drawSubgrid(bounds image.Rectangle, style gridcomponent.Style) {
	keyRunes := []rune("ASDFGHJKL")
	if o.sublayerKeys != "" {
		keyRunes = []rune(strings.ToUpper(o.sublayerKeys))
	}
	maxKeys := min(len(keyRunes), subgridCols*subgridRows)

	xBreaks := make([]int, subgridCols+1)
	yBreaks := make([]int, subgridRows+1)
	xBreaks[0] = bounds.Min.X
	yBreaks[0] = bounds.Min.Y
	for i := 1; i <= subgridCols; i++ {
		xBreaks[i] = bounds.Min.X + int(
			float64(i)*float64(bounds.Dx())/float64(subgridCols)+subgridHalfPixel,
		)
	}
	for i := 1; i <= subgridRows; i++ {
		yBreaks[i] = bounds.Min.Y + int(
			float64(i)*float64(bounds.Dy())/float64(subgridRows)+subgridHalfPixel,
		)
	}
	xBreaks[subgridCols] = bounds.Max.X
	yBreaks[subgridRows] = bounds.Max.Y

	index := 0
	for row := range subgridRows {
		for col := range subgridCols {
			if index >= maxKeys {
				break
			}
			cell := image.Rect(
				xBreaks[col],
				yBreaks[row],
				xBreaks[col+1],
				yBreaks[row+1],
			)
			o.drawRect(cell, subgridBackground, style.LineColor, subgridLineWidth)
			o.drawTextCentered(
				string(keyRunes[index]),
				cell,
				style.LabelFontName,
				style.LabelFontSize*subgridFontScale,
				style.LabelFontColor,
			)
			index++
		}
	}
}

func (o *x11Overlay) drawRect(
	bounds image.Rectangle,
	fill uint32,
	border uint32,
	lineWidth float64,
) {
	C.neru_x11_overlay_rect(
		o.raw,
		C.double(bounds.Min.X),
		C.double(bounds.Min.Y),
		C.double(bounds.Dx()),
		C.double(bounds.Dy()),
		C.uint(fill),
		C.uint(border),
		C.double(lineWidth),
	)
}

func (o *x11Overlay) drawTextCentered(
	text string,
	bounds image.Rectangle,
	fontFamily string,
	fontSize float64,
	color uint32,
) {
	cText := C.CString(text)
	cFontFamily := C.CString(fontFamily)

	defer C.free(unsafe.Pointer(cText))       //nolint:nlreturn
	defer C.free(unsafe.Pointer(cFontFamily)) //nolint:nlreturn

	C.neru_x11_overlay_text(
		o.raw,
		cText,
		cFontFamily,
		C.double(bounds.Min.X+bounds.Dx()/2),
		C.double(bounds.Min.Y+bounds.Dy()/2),
		C.double(fontSize),
		C.uint(color),
	)
}

func (o *x11Overlay) drawLabelBackground(
	label string,
	cell image.Rectangle,
	style recursivegridcomponent.Style,
) {
	fontSize := style.LabelFontSize
	paddingX := resolveAutoPadding(fontSize, style.LabelBackgroundPaddingX, true)
	paddingY := resolveAutoPadding(fontSize, style.LabelBackgroundPaddingY, false)
	width := estimateTextWidth(label, fontSize) + paddingX*paddingMultiplier
	height := estimateTextHeight(fontSize) + paddingY*paddingMultiplier
	rect := centeredRect(cell, width, height)

	o.drawRect(
		rect,
		style.LabelBackgroundColor,
		style.LineColor,
		max(style.LabelBackgroundBorderWidth, 0),
	)
}

func (o *x11Overlay) drawSubKeyPreview(
	label string,
	cell image.Rectangle,
	style recursivegridcomponent.Style,
) {
	previewRect := image.Rect(
		cell.Min.X,
		cell.Max.Y-estimateTextHeight(style.SubKeyPreviewFontSize)-subKeyPreviewPaddingBottom,
		cell.Max.X,
		cell.Max.Y,
	)

	o.drawTextCentered(
		label,
		previewRect,
		style.LabelFontName,
		style.SubKeyPreviewFontSize,
		style.SubKeyPreviewTextColor,
	)
}
