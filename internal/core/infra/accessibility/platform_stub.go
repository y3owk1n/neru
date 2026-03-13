//go:build !darwin

package accessibility

import "image"

func platformActiveScreenBounds() image.Rectangle { return image.Rectangle{} }
