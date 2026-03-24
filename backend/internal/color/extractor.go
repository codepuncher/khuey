package color

import (
	"fmt"
	"image"
	"math"

	"github.com/disintegration/imaging"
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
	subsampleWidth int     // Target width for subsampling (32-128px)
	gammaCorrection float64 // Gamma correction value (typically 2.2)
}

// NewExtractor creates a new color extractor
func NewExtractor(subsampleWidth int, gamma float64) (*Extractor, error) {
	if subsampleWidth < 16 || subsampleWidth > 256 {
		return nil, fmt.Errorf("subsample width must be between 16 and 256, got %d", subsampleWidth)
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
func (e *Extractor) ExtractColors(img image.Image, zones []Zone) ([]ZoneColor, error) {
	if img == nil {
		return nil, fmt.Errorf("image is nil")
	}

	// Step 1: Subsample the image for performance (using INTER_AREA equivalent)
	subsampled := e.subsampleImage(img)

	// Step 2: Extract colors from each zone
	colors := make([]ZoneColor, 0, len(zones))
	
	for _, zone := range zones {
		color, err := e.extractZoneColor(subsampled, zone)
		if err != nil {
			return nil, fmt.Errorf("failed to extract color for zone %d: %w", zone.ID, err)
		}
		colors = append(colors, color)
	}

	return colors, nil
}

// subsampleImage rescales image to target width while maintaining aspect ratio
func (e *Extractor) subsampleImage(img image.Image) image.Image {
	bounds := img.Bounds()
	origWidth := bounds.Dx()
	
	// If image is already smaller, don't upscale
	if origWidth <= e.subsampleWidth {
		return img
	}

	// Calculate height maintaining aspect ratio
	aspect := float64(bounds.Dy()) / float64(origWidth)
	newHeight := int(float64(e.subsampleWidth) * aspect)

	// Use Lanczos resampling (similar to INTER_AREA for downscaling)
	return imaging.Resize(img, e.subsampleWidth, newHeight, imaging.Lanczos)
}

// extractZoneColor extracts the mean color from a zone
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

	// Extract subimage for this zone
	subImg := imaging.Crop(img, image.Rect(x1, y1, x2, y2))

	// Calculate mean color
	r, g, b := e.calculateMeanColor(subImg)

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

// calculateMeanColor computes the average RGB color of an image
func (e *Extractor) calculateMeanColor(img image.Image) (uint8, uint8, uint8) {
	bounds := img.Bounds()
	var rSum, gSum, bSum uint64
	var count uint64

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
