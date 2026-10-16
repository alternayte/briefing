package feed

// CalibrationRow is one story to rate, with its three rating forms.
type CalibrationRow struct {
	Headline string
	Summary  string
	Rated    string
	Buttons  []RateButtonProps
}
