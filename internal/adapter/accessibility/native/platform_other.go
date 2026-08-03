//go:build !darwin

package native

import "image"

func platformActiveScreenBounds() image.Rectangle { return image.Rectangle{} }
