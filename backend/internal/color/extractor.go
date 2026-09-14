package color

import (
	"fmt"
	"image"
	"math"
)

// Constants for color extraction
const (
	MinSubsampleWidth = 16  // Minimum subsample width for color extraction
	MaxSubsampleWidth = 256 // Maximum subsample width for color extraction
)

// Zone represents a rectangular region on screen with UV coordinates
type Zone struct {
	ID   int     // Zone identifier
	U1   float64 // Left (0.0-1.0)
	V1   float64 // Top (0.0-1.0)
	U2   float64 // Right (0.0-1.0)
	V2   float64 // Bottom (0.0-1.0)
	Name string  // Optional zone name
}

// ZoneColor represents the extracted color for a zone
type ZoneColor struct {
	ZoneID int
	R      uint8
	G      uint8
	B      uint8
}

// Extractor handles color extraction from images
type Extractor struct {
	subsampleWidth  int     // Target width for subsampling (32-128px)
	gammaCorrection float64 // Gamma correction value (typically 2.2)
}

// NewExtractor creates a new color extractor
func NewExtractor(subsampleWidth int, gamma float64) (*Extractor, error) {
	if subsampleWidth < MinSubsampleWidth || subsampleWidth > MaxSubsampleWidth {
		return nil, fmt.Errorf("subsample width must be between %d and %d, got %d", MinSubsampleWidth, MaxSubsampleWidth, subsampleWidth)
	}

	if gamma <= 0 {
		return nil, fmt.Errorf("gamma must be positive, got %f", gamma)
	}

	return &Extractor{
		subsampleWidth:  subsampleWidth,
		gammaCorrection: gamma,
	}, nil
}

// ExtractColors extracts average colors from zones in the image
// OPTIMIZED: Skip global subsampling, sample zones directly with stride
func (e *Extractor) ExtractColors(img image.Image, zones []Zone) ([]ZoneColor, error) {
	if img == nil {
		return nil, fmt.Errorf("image is nil")
	}

	// Extract colors from each zone directly (no global subsampling)
	// Much faster than resizing the entire image first
	colors := make([]ZoneColor, 0, len(zones))

	for _, zone := range zones {
		color, err := e.extractZoneColor(img, zone)
		if err != nil {
			return nil, fmt.Errorf("failed to extract color for zone %d: %w", zone.ID, err)
		}
		colors = append(colors, color)
	}

	return colors, nil
}

// extractZoneColor extracts the mean color from a zone using stride sampling
// OPTIMIZED: Sample pixels with stride instead of processing every pixel
func (e *Extractor) extractZoneColor(img image.Image, zone Zone) (ZoneColor, error) {
	bounds := img.Bounds()
	width := float64(bounds.Dx())
	height := float64(bounds.Dy())

	// Convert UV coordinates to pixel coordinates
	x1 := int(zone.U1 * width)
	y1 := int(zone.V1 * height)
	x2 := int(zone.U2 * width)
	y2 := int(zone.V2 * height)

	// Clamp to image bounds
	x1 = max(0, min(x1, bounds.Max.X-1))
	y1 = max(0, min(y1, bounds.Max.Y-1))
	x2 = max(x1+1, min(x2, bounds.Max.X))
	y2 = max(y1+1, min(y2, bounds.Max.Y))

	// Calculate stride based on zone size and target subsample width
	// For a 2560px wide screen with 64px target, stride ≈ 40px
	zoneWidth := x2 - x1

	// Calculate stride to get approximately subsampleWidth samples across zone
	stride := max(1, zoneWidth/e.subsampleWidth)

	// Calculate mean color with stride sampling (much faster)
	r, g, b := e.calculateMeanColorWithStride(img, x1, y1, x2, y2, stride)

	// Apply gamma correction
	r = e.applyGamma(r)
	g = e.applyGamma(g)
	b = e.applyGamma(b)

	return ZoneColor{
		ZoneID: zone.ID,
		R:      r,
		G:      g,
		B:      b,
	}, nil
}

// calculateMeanColorWithStride computes average RGB using stride sampling
// OPTIMIZED: Sample every Nth pixel instead of all pixels for speed
func (e *Extractor) calculateMeanColorWithStride(img image.Image, x1, y1, x2, y2, stride int) (uint8, uint8, uint8) {
	// At boxes a color.Color per sample. The sampling coordinates are 0-based,
	// so an image whose bounds don't cover the region keeps the At path, which
	// reads those samples as zero.
	if rgba, ok := img.(*image.RGBA); ok && image.Rect(x1, y1, x2, y2).In(rgba.Rect) {
		return meanRGBAWithStride(rgba, x1, y1, x2, y2, stride)
	}

	var rSum, gSum, bSum uint64
	var count uint64

	// Sample pixels with stride
	for y := y1; y < y2; y += stride {
		for x := x1; x < x2; x += stride {
			r, g, b, _ := img.At(x, y).RGBA()
			// RGBA returns values 0-65535, convert to 0-255
			rSum += uint64(r >> 8)
			gSum += uint64(g >> 8)
			bSum += uint64(b >> 8)
			count++
		}
	}

	if count == 0 {
		return 0, 0, 0
	}

	return uint8(rSum / count), uint8(gSum / count), uint8(bSum / count)
}

// meanRGBAWithStride is calculateMeanColorWithStride reading Pix directly.
func meanRGBAWithStride(img *image.RGBA, x1, y1, x2, y2, stride int) (uint8, uint8, uint8) {
	var rSum, gSum, bSum uint64
	var count uint64

	for y := y1; y < y2; y += stride {
		i := img.PixOffset(x1, y)
		for x := x1; x < x2; x += stride {
			rSum += uint64(img.Pix[i])
			gSum += uint64(img.Pix[i+1])
			bSum += uint64(img.Pix[i+2])
			count++
			i += stride * 4
		}
	}

	if count == 0 {
		return 0, 0, 0
	}

	return uint8(rSum / count), uint8(gSum / count), uint8(bSum / count)
}

// applyGamma applies gamma correction to a color channel
func (e *Extractor) applyGamma(value uint8) uint8 {
	// Normalize to 0-1
	normalized := float64(value) / 255.0

	// Apply gamma correction
	// For display->light: use 1/gamma
	// We're going from display to light, so use 1/gamma
	corrected := math.Pow(normalized, 1.0/e.gammaCorrection)

	// Convert back to 0-255
	return uint8(corrected * 255.0)
}
