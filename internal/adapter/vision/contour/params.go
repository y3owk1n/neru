package contour

import "github.com/y3owk1n/neru/internal/config"

// ParamsFromConfig maps the hints.contour section onto the detector's
// parameters. The timeout is not among them. The caller applies it to the context.
func ParamsFromConfig(cfg config.HintsContourConfig) Params {
	return Params{
		EdgeLowThreshold:  cfg.EdgeLowThreshold,
		EdgeHighThreshold: cfg.EdgeHighThreshold,
		MinTargetWidth:    cfg.MinTargetWidth,
		MinTargetHeight:   cfg.MinTargetHeight,
		MaxTargetWidth:    cfg.MaxTargetWidth,
		MaxTargetHeight:   cfg.MaxTargetHeight,
		FlatLineHeight:    cfg.FlatLineHeight,
		ContainerHeight:   cfg.ContainerHeight,
		SameCenterSlack:   cfg.SameCenterSlack,
		SquareIconSize:    cfg.SquareIconSize,
		SquareIconSlack:   cfg.SquareIconSlack,
	}
}
