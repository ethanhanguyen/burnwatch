package report

import (
	"encoding/json"

	"github.com/ethanhanguyen/burnwatch/analyze"
)

func FormatCalibrationJSON(report analyze.CalibrationReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}
