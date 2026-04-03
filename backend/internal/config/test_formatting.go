package config

// BadlyFormatted demonstrates hooks catching formatting issues
func BadlyFormatted() string {
	x := "no spaces"
	y := "too many spaces"
	if x == "test" {
		return y
	}
	return x
}
